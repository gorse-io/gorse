============
CoClustering
============

Co-Clustering [#COC]_ predicts ratings by clustering users and items. 

.. math::

    \hat r_{ij}=A^{COC}_{gh}+(A^R_i-A^{RC}_g)+(A^C_j-A^{CC}_h)


Training
========

References
==========

.. [#COC] George, Thomas, and Srujana Merugu. "A scalable collaborative filtering framework based on co-clustering." Data Mining, Fifth IEEE international conference on. IEEE, 2005.
