// Copyright 2022 gorse Project Authors
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

package search

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/base/task"
	"testing"
)

func TestIVFConfig(t *testing.T) {
	ivf := &IVF{}

	SetNumProbe(123)(ivf)
	assert.Equal(t, 123, ivf.numProbe)

	SetClusterErrorRate(0.123)(ivf)
	assert.Equal(t, float32(0.123), ivf.errorRate)

	SetIVFJobsAllocator(task.NewConstantJobsAllocator(234))(ivf)
	assert.Equal(t, 234, ivf.jobsAlloc.AvailableJobs())

	SetMaxIteration(345)(ivf)
	assert.Equal(t, 345, ivf.maxIter)
}
