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
In the sorting problem, predicting the score precisely actually is not important. Thus, the algorithm BPR optimize the model with the loss function AUC. For user :math:`u` and items :math:`i` and :math:`j`, the optimization objective function is

..math::

    \text{BPR-OPT}&=\ln p(\Theta|\hat r_{ui}>\hat r_{uj})\\
    &=\ln p(\hat r_{ui}>\hat r_{uj} | \Theta)p(\Theta)\\
    &=\ln \sigma(\hat r_{uij}) +\ln p(\Theta)\\
    &=\ln \sigma(\hat r_{uij})-\lambda \|\Theta\|^2

where :math:`\hat r_{uij}=\hat r_{ui}-\hat r_{uj}` is produced by :math:`\hat r_{ui}=\mathbb w_u^T\mathbb h_i=\sum_fw_{uf}h_{if}`. Thus, the method of updating parameters for BPR is as follows:

step 1: Sampling triples :math:`(u,i,j)` from training datasets, where :math:`i` is the positive sample(interacted with users :math:`u`) and :math:`j` is negative sample(no interaction with users :math:`u`)

step 2: Updating parameters, :math:`\Theta\leftarrow\Theta+\alpha\left(\left(1-\sigma(\hat r_{uij})\right)\frac{\partial \hat r_{uij}}{\partial \Theta}-\lambda\Theta\right)` 

Repeatedly execute the above steps until the algorithm converges, and the gradient of the parameters is

..math::

    \frac{\partial \hat r_{uij}}{\partial \theta}=\begin{cases}
    (h_{if}-h_{jf})&\text{if }\theta=w_{uf},\\
    w_{uf}&\text{if }\theta=h_{if},\\
    -w_{uf}&\text{if }\theta=h_{jf},\\
    0&\text{else}\end{cases}





WRMF
====
There are scenarios where we can't obtain the specific ratings of users, but the information similar to 'confidence level' can be got,such as Game duration, viewing duration and so on.
Although the time information can not directly reflect the user's preferences, it can show that users like it more likely.In this scenario, the user-item record can be expressed as a 
as confidence level :math:`r_{ui}` and a indicator :math:`p_{ui}` of  0-1(whether the user-item interaction exists), and if the user-item interaction does not exist, the confidence 
level is 0. 'Weighted' is to calculate the weight of loss corresponding to each record according to confidence. The objective function of optimization is 
..math::

    \min_{x_*,y_*}\sum_{u,i}c_{ui}(p_{ui}-x^T_uy_i)+\lambda\left(\sum_u\|x_u\|^2+\sum_i\|y_i\|^2\right)

Weights :math:`c_{ui}=1+\alpha\log(1+r_{ui}/\epsilon)` are calculated by confidence level. Because the interaction that does not occur also exists in the loss function, the 
conventional stochastic gradient descent has performance problems. Therefore, ALS is used to optimize the model. The training process is as follows:

step 1: Update each user's vector, :math:`x_u=(Y^TC^uY+\lambda I)^{-1}Y^TC^up(u)`

step 2: Update each item's vector, :math:`y_i=(X^TC^iX+\lambda I)^{-1}X^TC^ip(i)`

where :math:`Y\in\mathbb R^{n\times f}` and :math:`X\in\mathbb R^{m\times f}` represent matrix consisting of :math:`n` user vectors and :math:`m` item vectors respectively, and :math:`f` 
is the length of vector.  In :math:`C^u\in\mathbb R^{m\times m}`, only :math:`C^u_{i,i}=c_{ui}` on the diagonal line, on the other positions the value is 0, and  
:math:`C^i\in\mathbb R^{n \times n}` is similar. Meanwhile, :math:`p(u)=[p_{u1},\dots,p_{um}]^T` and :math:`p(i)=[p_{1i},\dots,p_{ni}]^T`.  :math:`C^u` and :math:`C^i` are so huge that they can 
be decomposed into :math:`Y^T C^uY = Y^T Y + Y ^T (C^ u − I)Y`. Lastly, most values of in :math:`C^u - I` is 0, so only the non-zero part needs to be calculated.