// Copyright 2026 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gorse-io/gorse/storage"
	"github.com/redis/go-redis/v9"
)

const redisGeoBatchSize = 512

// Encode the namespace before adding the first hash tag. Raw prefixes such as
// "{}" would otherwise disable Redis Cluster's hash-tag extraction entirely.
func (r *Redis) geoKeyPrefix() string {
	return "gorse_geo_" + geoNameDigest(string(r.TablePrefix)) + ":"
}

// All three keys share the same first nonempty hash tag. Neither namespace nor
// index names can inject braces or key separators.
func (r *Redis) geoKeys(name string) ([]string, string) {
	prefix := string(r.TablePrefix)
	identity := make([]byte, 0, 16+len(prefix)+len(name))
	identity = binary.BigEndian.AppendUint64(identity, uint64(len(prefix)))
	identity = append(identity, prefix...)
	identity = binary.BigEndian.AppendUint64(identity, uint64(len(name)))
	identity = append(identity, name...)
	digest := sha256.Sum256(identity)
	base := fmt.Sprintf("%s{%x}:%s:", r.geoKeyPrefix(), digest, base64.RawURLEncoding.EncodeToString([]byte(name)))
	return []string{base + "meta", base + "index", base + "points"},
		"gorse-geo-v1:" + base64.RawURLEncoding.EncodeToString(identity)
}

// Lua checks every argument and existing payload in a block before writing:
// Redis does not roll back earlier commands when a script returns an error.
// JSON is checked but remains opaque on the write path; it is never re-encoded
// in Lua, which would lose the original Go float64 coordinate precision.
const redisGeoLua = `
local function fail(message)
    error('GORSE_GEO_' .. message, 0)
end
local function keytype(key)
    return redis.call('TYPE', key).ok
end
local function types()
    local m = keytype(KEYS[1])
    local z = keytype(KEYS[2])
    local h = keytype(KEYS[3])
    if (m ~= 'none' and m ~= 'string') or
       (z ~= 'none' and z ~= 'zset') or
       (h ~= 'none' and h ~= 'hash') then
        fail('CORRUPT key type')
    end
end
local function exists()
    local t = keytype(KEYS[1])
    if t == 'none' then fail('NOT_FOUND') end
    if t ~= 'string' then fail('CORRUPT metadata type') end
    if redis.call('GET', KEYS[1]) ~= ARGV[1] then fail('CORRUPT metadata') end
    types()
end
local function unpackpoint(value, id)
    if type(value) ~= 'string' then fail('CORRUPT missing payload') end
    local colon = string.find(value, ':', 1, true)
    if not colon then fail('CORRUPT payload frame') end
    local digits = string.sub(value, 1, colon - 1)
    if not string.match(digits, '^[1-9][0-9]*$') then fail('CORRUPT payload length') end
    local n = tonumber(digits)
    if not n or n < 15 or n > string.len(value) - colon - 1 then
        fail('CORRUPT payload length')
    end
    local member = string.sub(value, colon + 1, colon + n)
    local hex = string.sub(member, 1, 13)
    if not string.match(hex, '^[0-9a-f]+$') or string.len(hex) ~= 13 or
       string.sub(member, 14, 14) ~= ':' or string.sub(member, 15) ~= id then
        fail('CORRUPT payload member')
    end
    local payload = string.sub(value, colon + n + 1)
    local ok, point = pcall(cjson.decode, payload)
    if not ok or type(point) ~= 'table' or point.Id ~= id or
       type(point.Cell) ~= 'number' or point.Cell ~= tonumber(hex, 16) or
       type(point.Longitude) ~= 'number' or type(point.Latitude) ~= 'number' or
       not (point.Longitude >= -180 and point.Longitude <= 180) or
       not (point.Latitude >= -90 and point.Latitude <= 90) then
        fail('CORRUPT payload JSON')
    end
    return member, payload
end
local function oldpoint(id)
    local value = redis.call('HGET', KEYS[3], id)
    if not value then return false end
    local member = unpackpoint(value, id)
    local score = redis.call('ZSCORE', KEYS[2], member)
    if not score or tonumber(score) ~= 0 then fail('CORRUPT missing index member') end
    return member
end
`

var redisGeoCreate = redis.NewScript(redisGeoLua + `
types()
local value = redis.call('GET', KEYS[1])
if value then
    if value ~= ARGV[1] then fail('CORRUPT metadata') end
    fail('ALREADY_EXISTS')
end
if redis.call('EXISTS', KEYS[2], KEYS[3]) ~= 0 then fail('CORRUPT orphan keys') end
redis.call('SET', KEYS[1], ARGV[1], 'NX')
return 1
`)

var redisGeoCheck = redis.NewScript(redisGeoLua + `
exists()
return 1
`)

var redisGeoAdd = redis.NewScript(redisGeoLua + `
exists()
if (#ARGV - 1) % 2 ~= 0 or (#ARGV - 1) / 2 > 512 then fail('CORRUPT write batch') end
local old = {}
local members = {}
local seen = {}
for i = 2, #ARGV, 2 do
    local id = ARGV[i]
    if id == '' or seen[id] then fail('CORRUPT write id') end
    seen[id] = true
    members[i] = unpackpoint(ARGV[i+1], id)
    old[i] = oldpoint(id)
end
for i = 2, #ARGV, 2 do
    if old[i] then redis.call('ZREM', KEYS[2], old[i]) end
    redis.call('HSET', KEYS[3], ARGV[i], ARGV[i+1])
    redis.call('ZADD', KEYS[2], 0, members[i])
end
return 1
`)

var redisGeoDeletePoints = redis.NewScript(redisGeoLua + `
exists()
if #ARGV - 1 > 512 then fail('CORRUPT delete batch') end
local old = {}
local seen = {}
for i = 2, #ARGV do
    if ARGV[i] == '' or seen[ARGV[i]] then fail('CORRUPT delete id') end
    seen[ARGV[i]] = true
    old[i] = oldpoint(ARGV[i])
end
for i = 2, #ARGV do
    if old[i] then
        redis.call('ZREM', KEYS[2], old[i])
        redis.call('HDEL', KEYS[3], ARGV[i])
    end
end
return 1
`)

var redisGeoDrop = redis.NewScript(redisGeoLua + `
types()
local value = redis.call('GET', KEYS[1])
if value and value ~= ARGV[1] then fail('CORRUPT metadata') end
redis.call('DEL', KEYS[1], KEYS[2], KEYS[3])
return 1
`)

// A page and its payloads are read in the same script. The continuation is the
// full last member, not an offset, so concurrent deletions cannot skip a page.
var redisGeoRead = redis.NewScript(redisGeoLua + `
exists()
local members = redis.call('ZRANGE', KEYS[2], ARGV[2], ARGV[3], 'BYLEX', 'LIMIT', 0, 512)
local result = {''}
for _, member in ipairs(members) do
    local id = string.sub(member, 15)
    local value = redis.call('HGET', KEYS[3], id)
    local stored, payload = unpackpoint(value, id)
    if stored ~= member then fail('CORRUPT inconsistent member') end
    local score = redis.call('ZSCORE', KEYS[2], member)
    if not score or tonumber(score) ~= 0 then fail('CORRUPT member score') end
    result[#result+1] = member
    result[#result+1] = payload
    result[1] = member
end
return result
`)

func redisGeoError(name string, err error) error {
	if err == nil {
		return nil
	}
	// Redis may append script location details; use the unique error token,
	// not generic NOT_FOUND/EXISTS substrings from unrelated Redis errors.
	message := strings.TrimPrefix(err.Error(), "ERR ")
	if strings.HasPrefix(message, "GORSE_GEO_NOT_FOUND") {
		return fmt.Errorf("geo index %q %w", name, storage.ErrNotFound)
	}
	if strings.HasPrefix(message, "GORSE_GEO_ALREADY_EXISTS") {
		return fmt.Errorf("geo index %q %w", name, storage.ErrAlreadyExists)
	}
	return fmt.Errorf("geo index %q %w", name, err)
}

// AddGeoIndex explicitly creates an empty geographic index without a TTL.
func (r *Redis) AddGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	keys, identity := r.geoKeys(name)
	return redisGeoError(name, redisGeoCreate.Run(ctx, r.client, keys, identity).Err())
}

// AddGeoPoints upserts validated, last-write-wins points in bounded blocks.
func (r *Redis) AddGeoPoints(ctx context.Context, name string, points []GeoPoint) error {
	records, err := prepareGeoPoints(name, points)
	if err != nil {
		return err
	}
	keys, identity := r.geoKeys(name)
	// Encode the entire input before the first write as well as validating it.
	values := make([]any, 0, len(records)*2)
	for _, record := range records {
		payload, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("encode geo point %q %w", record.Id, err)
		}
		member := fmt.Sprintf("%013x:", record.Cell) + record.Id
		frame := strconv.Itoa(len(member)) + ":" + member + string(payload)
		values = append(values, record.Id, frame)
	}
	if len(values) == 0 {
		return redisGeoError(name, redisGeoCheck.Run(ctx, r.client, keys, identity).Err())
	}
	for start := 0; start < len(values); start += redisGeoBatchSize * 2 {
		end := min(start+redisGeoBatchSize*2, len(values))
		args := make([]any, 1, 1+end-start)
		args[0] = identity
		args = append(args, values[start:end]...)
		if err = redisGeoAdd.Run(ctx, r.client, keys, args...).Err(); err != nil {
			return redisGeoError(name, err)
		}
	}
	return nil
}

// SearchGeoPoints scans the shared cell ranges, then uses the shared exact
// spherical-distance and bounded-heap implementation to select results.
func (r *Redis) SearchGeoPoints(ctx context.Context, name string, query GeoQuery) ([]GeoResult, error) {
	query, err := normalizeGeoQuery(name, query)
	if err != nil {
		return nil, err
	}
	keys, identity := r.geoKeys(name)
	// The common search helper may short-circuit N == 0; existence still matters.
	if err = redisGeoCheck.Run(ctx, r.client, keys, identity).Err(); err != nil {
		return nil, redisGeoError(name, err)
	}
	results, err := searchGeoPoints(ctx, query, func(ctx context.Context, ranges []geoRange, yield func(geoRecord) error) error {
		for _, interval := range ranges {
			lower := fmt.Sprintf("[%013x:", interval.Min)
			upper := "+"
			if interval.Max < (1<<52)-1 {
				upper = fmt.Sprintf("(%013x:", interval.Max+1)
			}
			for {
				page, err := redisGeoRead.Run(ctx, r.client, keys, identity, lower, upper).Slice()
				if err != nil {
					return redisGeoError(name, err)
				}
				if len(page) == 0 || len(page)%2 != 1 || len(page) > 1+2*redisGeoBatchSize {
					return fmt.Errorf("geo index %q corrupt range response", name)
				}
				last, ok := page[0].(string)
				if !ok || (len(page) > 1 && last == "") {
					return fmt.Errorf("geo index %q corrupt range continuation", name)
				}
				for i := 1; i < len(page); i += 2 {
					member, memberOK := page[i].(string)
					payload, payloadOK := page[i+1].(string)
					if !memberOK || !payloadOK {
						return fmt.Errorf("geo index %q corrupt point response", name)
					}
					var record geoRecord
					if err = json.Unmarshal([]byte(payload), &record); err != nil {
						return fmt.Errorf("decode geo index %q %w", name, err)
					}
					canonical, err := prepareGeoPoints(name, []GeoPoint{record.GeoPoint})
					if err != nil {
						return fmt.Errorf("decode geo index %q %w", name, err)
					}
					if len(canonical) != 1 || canonical[0] != record || record.Cell < interval.Min || record.Cell > interval.Max ||
						member != fmt.Sprintf("%013x:", record.Cell)+record.Id {
						return fmt.Errorf("geo index %q corrupt point coordinates or member", name)
					}
					if err = yield(record); err != nil {
						return err
					}
				}
				if len(page) < 1+2*redisGeoBatchSize {
					break
				}
				lower = "(" + last
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// DeleteGeoPoints removes IDs without implicitly creating an index.
func (r *Redis) DeleteGeoPoints(ctx context.Context, name string, ids ...string) error {
	ids, err := prepareGeoIDs(name, ids)
	if err != nil {
		return err
	}
	keys, identity := r.geoKeys(name)
	if len(ids) == 0 {
		return redisGeoError(name, redisGeoCheck.Run(ctx, r.client, keys, identity).Err())
	}
	for start := 0; start < len(ids); start += redisGeoBatchSize {
		end := min(start+redisGeoBatchSize, len(ids))
		args := make([]any, 1, 1+end-start)
		args[0] = identity
		for _, id := range ids[start:end] {
			args = append(args, id)
		}
		if err = redisGeoDeletePoints.Run(ctx, r.client, keys, args...).Err(); err != nil {
			return redisGeoError(name, err)
		}
	}
	return nil
}

// DeleteGeoIndex atomically drops all three keys; absence is successful.
func (r *Redis) DeleteGeoIndex(ctx context.Context, name string) error {
	if err := validateGeoName(name); err != nil {
		return err
	}
	keys, identity := r.geoKeys(name)
	return redisGeoError(name, redisGeoDrop.Run(ctx, r.client, keys, identity).Err())
}

// Geographic keys use an encoded namespace, independent from the raw KV prefix.
// Purge visits this namespace on each master and deletes one key per command so
// the pipeline remains valid across different indexes' Cluster slots.
func (r *Redis) purgeGeoIndexes(ctx context.Context, client redis.UniversalClient) error {
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, r.geoKeyPrefix()+"*", redisGeoBatchSize).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			pipeline := client.Pipeline()
			for _, key := range keys {
				pipeline.Del(ctx, key)
			}
			if _, err = pipeline.Exec(ctx); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
