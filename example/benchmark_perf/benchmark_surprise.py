import sys
import time

from surprise import Dataset
from surprise import SVD
from surprise.model_selection import cross_validate

models = {
    'ml-100k':
        SVD(n_factors=50, n_epochs=100,
            init_mean=0, init_std_dev=0.001,
            lr_all=0.01, reg_all=0.1, verbose=True),
    'ml-1m':
        SVD(n_factors=80, n_epochs=100,
            init_mean=0, init_std_dev=0.001,
            lr_all=0.005, reg_all=0.05, verbose=True)
}

if __name__ == '__main__':
    # Show usage
    if len(sys.argv) == 1:
        print('usage: %s [ml-100k|ml-1m]' % sys.argv[0])
        exit(0)
    dataset = sys.argv[1]
    # Load the movielens-100k dataset (download it if needed).
    data = Dataset.load_builtin('ml-100k')
    # Use the famous SVD algorithm.
    algo = models[dataset]
    # Run 5-fold cross-validation and print results.
    start = time.time()
    cross_validate(algo, data, measures=['RMSE', 'MAE'], cv=5, verbose=True)
    end = time.time()
    print('time = %fs' % (end - start))
