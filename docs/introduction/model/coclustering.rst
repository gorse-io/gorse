============
CoClustering
============

CoClustering [#COC]_ predicts ratings by clustering users and items. 

Definition
==========

Let :math:`\mathcal{U} = \{u\}^m_{u=1}` be the set of users such that :math:`|\mathcal{U}|=m` and :math:`\mathcal{P} = \{i\}^n_{i=1}` be the set of items such that :math:`|\mathcal{P}|=n`. Let :math:`A` be the :math:`m\times n` ratings matrix such that :math:`A_{ui}` is the rating of the user :math:`u`for the item :math:`i` and let :math:`W` be the :math:`m\times n` matrix corresponding to the confidence of the ratings in :math:`A`. In the absence of explicit confidence information, we assume :math:`W_{ij} = 1` when the rating is known and :math:`0` otherwise.

Let :math:`\rho:\{1,\cdots,m\} \rightarrow \{1,\cdots,k\}` and :math:`\gamma:\{1,\cdots,n\} \rightarrow \{1,\cdots,l\}` denote the user and item clustering where :math:`k` and :math:`l` are number of user and item clusters.

.. math::

    \hat{A}_{ui}=A^{COC}_{gh}+(A^R_u-A^{RC}_g)+(A^C_i-A^{CC}_h)

where :math:`g=\rho(u)`, :math:`h=\gamma(i)` and :math:`A^R_u`, :math:`A^C_i` are the average ratings of user :math:`u` and item :math:`i`, and :math:`A^{COC}_{gh}`, :math:`A^R_g` and :math:`A^{CC}_h` are the average ratings of the corresponding cocluster, user-cluster and item-cluster respectively, i.e.,

.. math::

    A_{g h}^{C O C}=\frac{\sum_{u^{\prime} | \rho\left(u^{\prime}\right)=g} \sum_{i^{\prime} | \gamma\left(i^{\prime}\right)=h} A_{u^{\prime} i^{\prime}}}{\sum_{u^{\prime} | \rho\left(u^{\prime}\right)=g} \sum_{i^{\prime} | \gamma\left(i^{\prime}\right)=h} W_{u^{\prime} i^{\prime}}}

.. math::

    A_{g}^{R C}=\frac{\sum_{u^{\prime} | \rho\left(u^{\prime}\right)=g} \sum_{i^{\prime}=1}^{n} A_{u^{\prime} i^{\prime}}}{\sum_{u^{\prime} | \rho\left(u^{\prime}\right)=g} \sum_{i^{\prime}=1}^{n} W_{u^{\prime} i^{\prime}}}

.. math::

    A_{h}^{C C}=\frac{\sum_{u^{\prime}=1}^{m} \sum_{i^{\prime} | \gamma\left(i^{\prime}\right)=h} A_{u^{\prime} i^{\prime}}}{\sum_{u^{\prime}=1}^{m} \sum_{i^{\prime} | \gamma\left(i^{\prime}\right)=h} W_{u^{\prime} i^{\prime}}}

.. math::

    A_{u}^{R}=\frac{\sum_{i^{\prime}=1}^{n} A_{u i^{\prime}}}{\sum_{i^{\prime}=1}^{n} W_{u i^{\prime}}} A_{i}^{C}=\frac{\sum_{u^{\prime}=1}^{m} A_{u^{\prime} i}}{\sum_{u^{\prime}=1}^{m} W_{u^{\prime} i}}

Using :math:`\hat A`, we can now pose the prediction of unknown ratings as a co-clustering problem where we seek to find the optimal user and item clustering :math:`(\rho,\gamma)` such that the approximation error of :math:`A` (which is a function of :math:`(\rho,\gamma)`) with respect to the known ratings of :math:`A` is minimized, i.e.,

.. math::

    \min _{(\rho, \gamma)} \sum_{u=1}^{m} \sum_{i=1}^{n} W_{u i}\left(A_{u i}-\hat{A}_{u i}\right)^{2}

Training
========

**Input:**  Ratings Matrix A , Non-zeros matrix W , #row clusters :math:`l`, #col. clusters :math:`k`.

**Output:** Locally optimal co-clustering :math:`(\rho,\gamma)` and averages :math:`A^{COC}`, :math:`A^{RC}`, :math:`A^{CC}`, :math:`A^R` and :math:`A^C`.

**Method:**

- Randomly initialize :math:`(\rho,\gamma)`.

- **repeat**

  - Compute averages :math:`A^{COC}`, :math:`A^{RC}`, :math:`A^{CC}`, :math:`A^R` and :math:`A^C`.

  - Update row cluster assignments

    .. math::

        \rho(u)=\underset{1 \leq g \leq k}{\operatorname{argmin}} \sum_{i=1}^{n} W_{u i} \left( A_{u i} - A_{g \gamma(j)}^{C O C} -A_{i}^{R}+A_{g}^{R C} -A_{j}^{C}+A_{\gamma(j)}^{C C}  \right)^{2}, \quad 1 \leq i \leq m

  - Update column cluster assignments

    .. math::

        \gamma(j)=\underset{1 \leq h \leq l}{\operatorname{argmin}} \sum_{i=1}^{m} W_{i j}\left(A_{i j}-A_{\rho(i) h}^{C O C}-A_{i}^{R}+A_{\rho(i)}^{R C}-A_{j}^{C}+A_{h}^{C C} \right)^{2}, \quad 1 \leq j \leq n

- **until** *convergence*

Prediction
==========

**Input:**  :math:`A^{COC}`, :math:`A^{RC}`, :math:`A^{CC}`, :math:`A^R` , :math:`A^C`, :math:`A^{glob}`, co-clustering :math:`(\rho,\gamma)` user set :math:`\mathcal U`, item set :math:`\mathcal P`, user :math:`u`, item :math:`i`.

**Output:** Rating :math:`r`

**Method:**

- Case :math:`u\in\mathcal U` and :math:`i\in\mathcal P`: // old user-old item

.. math::

    g \leftarrow \rho(i), h \leftarrow \gamma(j)

.. math::

    r \leftarrow A_{i}^{R}+A_{j}^{C}-A_{g}^{R C}-A_{h}^{C C}+A_{g h}^{C o C}

- Case :math:`u\in\mathcal U` and :math:`i\notin\mathcal P`: // old user-new item

.. math::

    g \leftarrow \rho(i), r \leftarrow A_{i}^{R}

- Case :math:`u\notin\mathcal U` and :math:`i\in\mathcal P`: // new user-old item

.. math::

    h \leftarrow \gamma(j), r \leftarrow A_{j}^{C}

- Case :math:`u\notin\mathcal U` and :math:`i\notin\mathcal P`: // new user-new item

.. math::

    r \leftarrow A^{g l o b}

References
==========

.. [#COC] George, Thomas, and Srujana Merugu. "A scalable collaborative filtering framework based on co-clustering." Data Mining, Fifth IEEE international conference on. IEEE, 2005.
