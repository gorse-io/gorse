====================
Matrix Factorization
====================

SVD
===

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

SVD++
=====

NMF
===

BPR
===

WRMF
====
