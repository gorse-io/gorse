======
Models
======


+----------------------+----------+-------------------+
| Model                | Data     | Task              |
+======================+==========+===================+
| BaseLine             | explicit | prediction        |
+----------------------+----------+-------------------+
| NMF [#NMF]_          | explicit | prediction        |
+----------------------+----------+-------------------+
| SVD                  | explicit | prediction        |
| SVD++                | explicit | prediction        |
+----------------------+----------+-------------------+
| KNN [#KNN]_          | explicit | prediction        |
+----------------------+----------+-------------------+
| CoClustering [#COC]_ | explicit | prediction        |
+----------------------+----------+-------------------+
| SlopeOne [#SO]_      | explicit | prediction        |
+----------------------+----------+-------------------+
| ItemPop              | implicit | ranking/pointwise |
+----------------------+----------+-------------------+
| WRMF [#WRMF]_        | implicit | ranking/pointwise |
+----------------------+----------+-------------------+
| BPR [#BPR]_          | implicit | ranking/pairwise  |
+----------------------+----------+-------------------+

Apparently, these models using implicit feedbacks are more general since explicit feedbacks could be converted to implicit feedbacks and item ranking could be done by rating prediction.

Rating Prediction
=================

Matrix Factorization
--------------------

In matrix factorization, the matrix $R$ is modeled to be a multiplication of two matrices with lower ranks:

.. math::

    R=PQ^T

where $P\in\mathcal{R}^{|U|\times k}$ and $Q\in\mathcal{R}^{|V|\times k}$. $k$ is the number of factors. Two matrix factorization models are introduced in this section. An element in $R$ could be represented by

.. math::

    \hat r_{ij}=\mu+b_i+b_j+p_i^Tq_j

$\mu$ is the global mean, $b_i$ is the user bias and $b_j$ is the item bias. $p_i$ is the user factor which is the $i$-th row in $P$, $q_j$ is the item factor which is the $j$-th row in $Q$. 

The square error loss function with L2 regularization for a single element is

.. math::

    \mathcal L=(\hat r_{ij}- r_{ij})^2+\lambda\left(b_i^2+b_j^2+\|p_i\|^2+\|q_j\|^2\right)


Since most elements in $R$ are missing, the stochastic gradient descent is used to estimate parameters \cite{funk2006netflix}. Given a training sample $<i,j,r_{ij}>$ and the learning rate $\mu$, parameters are updated by

.. math::

    b&\leftarrow b-\mu(\hat r_{ij}-r_{ij})\\
    b_i&\leftarrow b_i-\mu\left((\hat r_{ij}-r_{ij})+\lambda b_i\right)\\
    b_j&\leftarrow b_j-\mu\left((\hat r_{ij}-r_{ij})+\lambda b_j\right)\\
    p_i&\leftarrow p_i-\mu\left((\hat r_{ij}-r_{ij})q_j+\lambda p_i\right)\\
    q_j&\leftarrow q_j-\mu\left((\hat r_{ij}-r_{ij})p_i+\lambda q_j\right)

KNN
---

Neighbors-based models [#KNN]_ predict ratings from similar items or similar users. There are two kinds of neighbors-based models: user-based and item-based, which means whether predictions are made by similar users or items. In general, item-based model is better than user-based model since items' characters are more consistent than users' preferences.

Similarity
^^^^^^^^^^

Similarity metrics define the definition of nearest neighbors. Ratings of two different user a and b (or item a and b) are represented by $D_a$ and $D_b$. Common ratings are $D_a\cap D_b=\{<k,r_{ak},r_{bk}>\mid<k,r_{ak}>\in D_a, <k,r_{bk}>\in D_b\}$. There are several most used similarity functions:

Cosine
""""""

.. math::

    \cos(a,b)=\frac{\sum\limits_{k\in|I_a\cap I_b|}r_{ak}\cdot r_{bk}}{\sqrt{\sum\limits_{k\in|I_a\cap I_b|}r_{ak}^2}\cdot\sqrt{\sum\limits_{k\in|I_a\cap I_b|}r_{bk}^2}}


Pearson
"""""""

Pearson similarity is similar to cosine similarity but ratings are subtracted by means first.

.. math::

    \text{pearson}(a,b)=\frac{\sum\limits_{k\in|I_a\cap I_b|}(r_{ak}-\tilde r_a)\cdot (r_{bk}-\tilde r_b)}{\sqrt{\sum\limits_{k\in|I_a\cap I_b|}(r_{ak}-\tilde r_a)^2}\cdot\sqrt{\sum\limits_{k\in|I_a\cap I_b|}(r_{bk}-\tilde r_b)^2}}

where :math:`\tilde r_a` is the mean of ratings rated by the user :math:`a`:

.. math::

    \tilde r_a = \sum_{k\in I_a} r_{ak}

Mean Square Distance
""""""""""""""""""""


The *Mean Square Distance* is

.. math::

    \text{msd}(a,b)=\frac{1}{|I_a\cap I_b|}\sum_{k\in|I_a\cap I_b|}(r_{ak}-r_{bk})^2

Then, the *Mean Square Distance Similarity* is

.. math::

    \text{msd_sim}(u, v) = \frac{1}{\text{msd}(u, v) + 1}


Predict
^^^^^^^

A rating could be predict by k nearest neighbors $\mathcal N_k(i)$ (k users or items with max k similarity).

.. math::

    \hat r_{ui}=\frac{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)r_{vi}}{\sum_{v\in \mathcal N_k(u)}\text{sim}(u,v)}


The basic KNN prediction has some problem, There are more advanced methods which achieve higher accuracy.

\begin{itemize}
    \item \textbf{KNN with means}: $\tilde r_l$ is the mean of l-th user's (or item's) ratings.

.. math::

    \hat r_{ij}=\tilde r_i+\frac{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)(r_{lj}-\tilde r_l)}{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)}
    
\item \textbf{KNN with Z-score}: $\sigma(r_i)$ is the standard deviation of l-th user's (or item's) ratings.

.. math::

    \hat r_{ij}=\tilde r_i+\sigma(r_i)\frac{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)\frac{r_{lj}-\tilde r_l}{\sigma(r_l)}}{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)}


\end{equation}
    \item \textbf{KNN with baseline}: $b_l$ is the baseline comes from the baseline model $\hat r_{ij}=b+b_i+b_j+p_i^Tq_j$.

.. math::

    \hat r_{ij}=b_i+\frac{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)(r_{lj}- b_l)}{\sum_{l\in \mathcal N_k(i)}\text{sim}(i,l)}


\end{equation}
\end{itemize}

The KNN model with baseline is the best model since biases are used.

CoClustering
------------

Co-Clustering [#COC]_ predicts ratings by clustering users and items. 

Slope One
---------

Slope One [#SO]_ predicts ratings by deviations between ratings of each pair of items.

References
==========

1. Hug, Nicolas. Surprise, a Python library for recommender systems. http://surpriselib.com, 2017.

1. G. Guo, J. Zhang, Z. Sun and N. Yorke-Smith, LibRec: A Java Library for Recommender Systems, in Posters, Demos, Late-breaking Results and Workshop Proceedings of the 23rd Conference on User Modelling, Adaptation and Personalization (UMAP), 2015.

.. [#NMF] Luo, Xin, et al. "An efficient non-negative matrix-factorization-based approach to collaborative filtering for recommender systems." IEEE Transactions on Industrial Informatics 10.2 (2014): 1273-1284.

.. [#SO] "Slope one predictors for online rating-based collaborative filtering." Proceedings of the 2005 SIAM International Conference on Data Mining. Society for Industrial and Applied Mathematics, 2005.

.. [#COC] George, Thomas, and Srujana Merugu. "A scalable collaborative filtering framework based on co-clustering." Data Mining, Fifth IEEE international conference on. IEEE, 2005.

1. Guo, Guibing, Jie Zhang, and Neil Yorke-Smith. "A Novel Bayesian Similarity Measure for Recommender Systems." IJCAI. 2013.

.. [#WRMF] Hu, Yifan, Yehuda Koren, and Chris Volinsky. "Collaborative filtering for implicit feedback datasets." Data Mining, 2008. ICDM'08. Eighth IEEE International Conference on. Ieee, 2008.

1. Massa, Paolo, and Paolo Avesani. "Trust-aware recommender systems." Proceedings of the 2007 ACM conference on Recommender systems. ACM, 2007.

.. [#KNN] Desrosiers, Christian, and George Karypis. "A comprehensive survey of neighborhood-based recommendation methods." Recommender systems handbook. Springer, Boston, MA, 2011. 107-144.

1. Koren, Yehuda. "Factorization meets the neighborhood: a multifaceted collaborative filtering model." Proceedings of the 14th ACM SIGKDD international conference on Knowledge discovery and data mining. ACM, 2008.

.. [#BPR] Rendle, Steffen, et al. "BPR: Bayesian personalized ranking from implicit feedback." Proceedings of the twenty-fifth conference on uncertainty in artificial intelligence. AUAI Press, 2009.

1. Rendle, Steffen. "Factorization machines." 2010 IEEE International Conference on Data Mining. IEEE, 2010.