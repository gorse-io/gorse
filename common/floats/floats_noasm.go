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

var impl implementation

type implementation struct{}

func (implementation) mulConstAddTo(a []float32, b float32, c []float32) {
	mulConstAddTo(a, b, c)
}

func (implementation) mulConstTo(a []float32, b float32, c []float32) {
	mulConstTo(a, b, c)
}

func (implementation) addConst(a []float32, b float32) {
	addConst(a, b)
}

func (implementation) sub(a, b []float32) {
	sub(a, b)
}

func (implementation) subTo(a, b, c []float32) {
	subTo(a, b, c)
}

func (implementation) mulTo(a, b, c []float32) {
	mulTo(a, b, c)
}

func (implementation) mulConst(a []float32, b float32) {
	mulConst(a, b)
}

func (implementation) divTo(a, b, c []float32) {
	divTo(a, b, c)
}

func (implementation) sqrtTo(a, b []float32) {
	sqrtTo(a, b)
}

func (implementation) dot(a, b []float32) float32 {
	return dot(a, b)
}

func (implementation) euclidean(a, b []float32) float32 {
	return euclidean(a, b)
}
