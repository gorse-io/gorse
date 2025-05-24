//go:build noasm || (!amd64 && !arm64)

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

package floats

type Feature uint64

var feature Feature

func (Feature) String() string {
	return "NOASM"
}

func (Feature) mulConstAddTo(a []float32, b float32, c, dst []float32) {
	mulConstAddTo(a, b, c, dst)
}

func (Feature) mulConstAdd(a []float32, b float32, c []float32) {
	mulConstAdd(a, b, c)
}

func (Feature) mulConstTo(a []float32, b float32, c []float32) {
	mulConstTo(a, b, c)
}

func (Feature) addConst(a []float32, b float32) {
	addConst(a, b)
}

func (Feature) sub(a, b []float32) {
	sub(a, b)
}

func (Feature) subTo(a, b, c []float32) {
	subTo(a, b, c)
}

func (Feature) mulTo(a, b, c []float32) {
	mulTo(a, b, c)
}

func (Feature) mulConst(a []float32, b float32) {
	mulConst(a, b)
}

func (Feature) divTo(a, b, c []float32) {
	divTo(a, b, c)
}

func (Feature) sqrtTo(a, b []float32) {
	sqrtTo(a, b)
}

func (Feature) dot(a, b []float32) float32 {
	return dot(a, b)
}

func (Feature) euclidean(a, b []float32) float32 {
	return euclidean(a, b)
}

func (Feature) mm(a, b, c []float32, m, n, k int, transA, transB bool) {
	mm(a, b, c, m, n, k, transA, transB)
}
