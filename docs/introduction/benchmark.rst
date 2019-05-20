==========
Benchamrks
==========

Cross Validation
================

MovieLens 100K
--------------

- [SVD](https://godoc.org/github.com/zhenghaoz/gorse/model#SVD)

========== ======= ===== =======
Software   Memory  Time  RMSE
========== ======= ===== =======
gorse      33.9MB  13s   0.90795
librec     226.1MB 14s   0.90597
Surprise   83.4MB  1m12s 0.90720
========== ======= ===== =======

MovieLens 1M
------------

- [SVD](https://godoc.org/github.com/zhenghaoz/gorse/model#SVD)


======== ======= ===== =======
Software Memory  Time  RMSE
======== ======= ===== =======
gorse    260.5MB 3m17s 0.84272
librec   1.3GB   3m10s 0.84309
Surprise 597.6MB 16m8s 0.84300 
======== ======= ===== =======


Rating Prediction
=================

MovieLens 100K
--------------

============ ======= ======= =======
Model        RMSE    MAE     Time   
============ ======= ======= =======
SlopeOne     0.94204 0.74226 0:00:01
CoClustering 0.95896 0.75175 0:00:00
KNN          0.92556 0.72526 0:00:06
SVD          0.90728 0.71508 0:00:10
SVD++        0.90384 0.70952 0:00:21
============ ======= ======= =======

MovieLens 1M
------------

============ ======= ======= ======= =======
Model        RMSE    MAE     Time    (AVX2)
============ ======= ======= ======= =======
SlopeOne     0.90683 0.71541 0:00:26 
CoClustering 0.90701 0.71212 0:00:08 
KNN          0.86462 0.67663 0:02:07 
SVD          0.84252 0.66189 0:02:21 0:01:48
SVD++        0.84194 0.66156 0:03:39 0:02:47
============ ======= ======= ======= =======

Item Ranking
============

MovieLens 100K
--------------

======= ============ ========= ======= ======= ======= =======
Model   Precision@10 Recall@10 MAP@10  NDCG@10 MRR@10  Time
======= ============ ========= ======= ======= ======= =======
ItemPop 0.19081      0.11584   0.05364 0.21785 0.40991 0:00:03
SVD     0.32083      0.20906   0.11848 0.37643 0.59818 0:00:13
WRMF    0.34727      0.23665   0.14550 0.41614 0.65439 0:00:14
======= ============ ========= ======= ======= ======= =======

-- [#Surprise] Surprise benchmark: https://github.com/NicolasHug/Surprise#benchmarks

-- [#Librec] Librec benchmark: https://www.librec.net/release/v1.3/example.html
