======
Models
======

There are all models implemented in *gorse*:

+-------------------------+------------------------------+------------------+---------------------+
| Model                   | Data                         | Task             | Multi-threading Fit |
+-------------------------+----------+----------+--------+--------+---------+                     +
|                         | explicit | implicit | weight | rating | ranking |                     |
+=========================+==========+==========+========+========+=========+=====================+
| BaseLine                | Yes      |          |        | Yes    | Yes     | No                  |
+-------------------------+----------+----------+--------+--------+---------+---------------------+
| NMF [#NMF]_             | Yes      |          |        | Yes    | Yes     | No                  |
+-------------------------+----------+----------+--------+--------+---------+---------------------+
| SVD                     | Yes      |          |        | Yes    | Yes     | No                  |
+-------------------------+----------+----------+--------+--------+---------+---------------------+
| SVD++ [#SVDPP]_         | Yes      |          |        | Yes    | Yes     | Yes                 |
+-------------------------+----------+----------+--------+--------+---------+---------------------+
| KNN [#KNN]_             | Yes      |          |        | Yes    | Yes     | Yes                 |
+-------------------------+----------+----------+--------+--------+---------+---------------------+
| CoClustering [#COC]_    | Yes      |          |        | Yes    | Yes     | Yes                 |
+-------------------------+----------+----------+--------+--------+---------+---------------------+
| SlopeOne [#SO]_         | Yes      |          |        | Yes    | Yes     | Yes                 |
+-------------------------+----------+----------+--------+--------+---------+---------------------+
| ItemPop                 | Yes      | Yes      |        |        | Yes     | No                  |
+-------------------------+----------+----------+--------+--------+---------+---------------------+
| KNN (Implicit) [#WRMF]_ | Yes      |          |        | Yes    | Yes     | Yes                 |
+-------------------------+----------+----------+--------+--------+---------+---------------------+
| WRMF [#WRMF]_           | Yes      | Yes      | Yes    |        | Yes     | No                  |
+-------------------------+----------+----------+--------+--------+---------+---------------------+
| BPR [#BPR]_             | Yes      | Yes      |        |        | Yes     | No                  |
+-------------------------+----------+----------+--------+--------+---------+---------------------+

Apparently, these models using implicit feedbacks are more general since explicit feedbacks could be converted to implicit feedbacks and item ranking could be done by rating prediction.


Non-Personalized Models
=======================

ItemPop
-------

The only non-personalized model is ItemPop. It always recommmends top K popular items to all users.

Personalized Models
===================

.. toctree::
   :caption: Personalized Models
   :maxdepth: 1

   matrix_factorization
   knn
   coclustering
   slopeone


References
==========

.. [#Surprise] Hug, Nicolas. Surprise, a Python library for recommender systems. http://surpriselib.com, 2017.

.. [#LibRec] G. Guo, J. Zhang, Z. Sun and N. Yorke-Smith, LibRec: A Java Library for Recommender Systems, in Posters, Demos, Late-breaking Results and Workshop Proceedings of the 23rd Conference on User Modelling, Adaptation and Personalization (UMAP), 2015.

.. [#NMF] Luo, Xin, et al. "An efficient non-negative matrix-factorization-based approach to collaborative filtering for recommender systems." IEEE Transactions on Industrial Informatics 10.2 (2014): 1273-1284.

.. [#SO] "Slope one predictors for online rating-based collaborative filtering." Proceedings of the 2005 SIAM International Conference on Data Mining. Society for Industrial and Applied Mathematics, 2005.

.. [#COC] George, Thomas, and Srujana Merugu. "A scalable collaborative filtering framework based on co-clustering." Data Mining, Fifth IEEE international conference on. IEEE, 2005.

.. [#WRMF] Hu, Yifan, Yehuda Koren, and Chris Volinsky. "Collaborative filtering for implicit feedback datasets." Data Mining, 2008. ICDM'08. Eighth IEEE International Conference on. Ieee, 2008.

.. [#KNN] Desrosiers, Christian, and George Karypis. "A comprehensive survey of neighborhood-based recommendation methods." Recommender systems handbook. Springer, Boston, MA, 2011. 107-144.

.. [#SVDPP] Koren, Yehuda. "Factorization meets the neighborhood: a multifaceted collaborative filtering model." Proceedings of the 14th ACM SIGKDD international conference on Knowledge discovery and data mining. ACM, 2008.

.. [#BPR] Rendle, Steffen, et al. "BPR: Bayesian personalized ranking from implicit feedback." Proceedings of the twenty-fifth conference on uncertainty in artificial intelligence. AUAI Press, 2009.
