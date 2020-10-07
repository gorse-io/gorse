// Copyright 2020 Zhenghao Zhang
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
package engine

import (
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/config"
	. "github.com/zhenghaoz/gorse/database"
	"github.com/zhenghaoz/gorse/model"
	"github.com/zhenghaoz/gorse/server"
	"log"
	"time"
)

func Main(instance *server.Instance) {
	// Start monitor
	updateDone, fitStart := make(chan interface{}), make(chan interface{})
	updateStart, fitDone := make(chan model.ModelInterface), make(chan model.ModelInterface)
	updateSignal, fitSignal := newMonitor(instance)
	update(instance, updateStart, updateDone)
	fit(instance, fitStart, fitDone)
	var ranker model.ModelInterface
	isUpdating, isTraining := false, false
	for {
		select {
		case <-updateSignal:
			if !isUpdating && ranker != nil {
				isUpdating = true
				updateStart <- ranker
			}
		case <-fitSignal:
			if !isTraining {
				isTraining = true
				fitStart <- nil
			}
		case <-updateDone:
			isUpdating = false
		case ranker = <-fitDone:
			isTraining = false
			updateStart <- ranker
		}
	}
}

func update(instance *server.Instance, start chan model.ModelInterface, done chan interface{}) {
	for {
		ranker := <-start
		base.Must(instance.DB.SetInt(LastUpdateFeedbackCount, base.MustInt(instance.DB.CountFeedback())))
		base.Must(instance.DB.SetInt(LastUpdateIgnoreCount, base.MustInt(instance.DB.CountIgnore())))
		base.Must(RefreshRecommends(instance.DB, ranker, instance.Config.Recommend.TopN, instance.Config.Recommend.UpdateJobs))
		start <- nil
	}
}

func fit(instance *server.Instance, start chan interface{}, done chan model.ModelInterface) {
	for {
		<-start
		base.Must(instance.DB.SetInt(LastFitFeedbackCount, base.MustInt(instance.DB.CountFeedback())))
		dataset, err := instance.DB.ToDataSet()
		if err != nil {
			log.Fatal(err)
		}
		params := instance.Config.Params.ToParams(*instance.MetaData)
		ranker := config.LoadModel(instance.Config.Recommend.Model, params)
		ranker.Fit(dataset, &base.RuntimeOptions{Verbose: true, FitJobs: instance.Config.Recommend.FitJobs})
		done <- ranker
	}
}

func newMonitor(instance *server.Instance) (update chan interface{}, train chan interface{}) {
	for {
		db := instance.DB
		config := instance.Config
		// Get last info
		lastFitFeedbackCount := base.MustInt(db.GetInt(LastFitFeedbackCount))
		lastPredictFeedbackCount := base.MustInt(db.GetInt(LastUpdateFeedbackCount))
		lastPredictIgnoreCount := base.MustInt(db.GetInt(LastUpdateIgnoreCount))
		// Get current info
		currentFeedbackCount := base.MustInt(db.CountFeedback())
		currentIgnoreCount := base.MustInt(db.CountIgnore())
		// Send signal
		if currentFeedbackCount-lastFitFeedbackCount > config.Recommend.FitThreshold {
			train <- nil
		}
		if currentFeedbackCount+currentIgnoreCount-(lastPredictFeedbackCount+lastPredictIgnoreCount) > config.Recommend.UpdateThreshold {
			update <- nil
		}
		time.Sleep(time.Duration(config.Recommend.CheckPeriod) * time.Minute)
	}
}
