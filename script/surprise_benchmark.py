'''This module runs a 5-Fold CV for all the algorithms (default parameters) on
the movielens datasets, and reports average RMSE, MAE, and total computation
time.  It is used for making tables in the README.md file'''

import datetime
import random
import time
import sys

import numpy as np
from surprise import BaselineOnly
from surprise import CoClustering
from surprise import Dataset
from surprise import KNNBaseline
from surprise import KNNBasic
from surprise import KNNWithMeans
from surprise import KNNWithZScore
from surprise import NMF
from surprise import NormalPredictor
from surprise import SVD
from surprise import SVDpp
from surprise import SlopeOne
from surprise.model_selection import KFold
from surprise.model_selection import cross_validate
from tabulate import tabulate


def benchmark(dataset='ml-100k'):
    # set RNG
    np.random.seed(0)
    random.seed(0)
    # The algorithms to cross-validate
    classes = (SVD, SVDpp, NMF, SlopeOne, KNNBasic, KNNWithMeans, KNNBaseline, KNNWithZScore,
               CoClustering, BaselineOnly, NormalPredictor)
    # Load data set
    data = Dataset.load_builtin(dataset)
    kf = KFold(random_state=0)  # folds will be the same for all algorithms.
    header = [dataset, 'RMSE', 'MAE', 'Time']
    table = []
    for klass in classes:
        start = time.time()
        out = cross_validate(klass(), data, ['rmse', 'mae'], kf)
        cv_time = str(datetime.timedelta(seconds=int(time.time() - start)))
        mean_rmse = '{:.3f}'.format(np.mean(out['test_rmse']))
        mean_mae = '{:.3f}'.format(np.mean(out['test_mae']))

        new_line = [klass.__name__, mean_rmse, mean_mae, cv_time]
        print(tabulate([new_line], tablefmt="plain"))  # print current algo perf
        print()
        table.append(new_line)
    print(tabulate(table, header, tablefmt="pipe"))


if __name__ == '__main__':
    if len(sys.argv) > 1:
        benchmark(sys.argv[1])
    benchmark()
