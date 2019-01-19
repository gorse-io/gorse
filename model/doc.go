/*

Package model provides models for item rating and ranking.

There are two kinds of models: rating model and ranking model. Although rating models could be used for ranking,
performance won't be guaranteed and even won't make sense, vice versa.

=> Item rating models include: Random, Baseline, SVD(Target=Regression), SVD++, NMF, KNN, SlopeOne, CoClustering

=> Item ranking models includes: ItemPop, WRMF, SVD(Target=BPR)

*/
package model
