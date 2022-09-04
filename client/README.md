# Gorse Client

Go SDK for Gorse recommender system.

> ⚠️⚠️⚠️ This SDK is unstable currently. APIs might be changed in later versions.

## Install

```bash
go get github.com/zhenghaoz/gorse/client@master
```

## Usage

```go
import "github.com/zhenghaoz/gorse/client"

func main() {
    gorse = client.NewGorseClient("http://127.0.0.1:8087", "api_key")
	resp, err := gorse.InsertFeedback([]client.Feedback{{
		FeedbackType: "read",
		UserId:       userId,
		Timestamp:    timestamp,
		ItemId:       "300",
	}})
}
```

## Test


In the root directory of Gorse source:

```bash
# Setup Gorse
docker-compose up -d

# Test
go test -tags='integrate_test' ./client/
```
