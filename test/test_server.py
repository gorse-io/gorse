import json
import math
import sys
import time
import urllib.request
from typing import Dict, Set, List

import pandas as pd

EPSILON = 0.05


def get(url: str) -> Dict:
    f = urllib.request.urlopen(url)
    text = f.read().decode('utf-8')
    return json.loads(text)


def precision(target_set: Set[int], top_list: List[int]) -> float:
    hit = 0
    for item_id in top_list:
        if item_id in target_set:
            hit += 1
    return hit / len(top_list)


def recall(target_set: Set[int], top_list: List[int]) -> float:
    hit = 0
    for item_id in top_list:
        if item_id in target_set:
            hit += 1
    return hit / len(target_set)


def ndcg(target_set: Set[int], top_list: List[int]) -> float:
    idcg = 0
    for i in range(min(len(target_set), len(top_list))):
        idcg += 1 / math.log2(i + 2)
    dcg = 0
    for i, item_id in enumerate(top_list):
        if item_id in target_set:
            dcg += 1 / math.log2(i + 2)
    return dcg / idcg


def eval_item_pop(url: str, users):
    prec_score, recall_score, ndcg_score = 0, 0, 0
    for user_id, user_df in users:
        response = get(url + 'popular/')
        top_list = [int(v['ItemId']) for v in response]
        target_set = set(user_df['item_id'])
        prec_score += precision(target_set, top_list)
        recall_score += recall(target_set, top_list)
        ndcg_score += ndcg(target_set, top_list)
    return prec_score / len(users), recall_score / len(users), ndcg_score / len(users)


def eval_svd_bpr(url: str, users):
    prec_score, recall_score, ndcg_score = 0, 0, 0
    for user_id, user_df in users:
        response = get(url + 'recommends/' + str(user_id))
        top_list = [int(v['ItemId']) for v in response]
        target_set = set(user_df['item_id'])
        prec_score += precision(target_set, top_list)
        recall_score += recall(target_set, top_list)
        ndcg_score += ndcg(target_set, top_list)
    return prec_score / len(users), recall_score / len(users), ndcg_score / len(users)


def test_server(server_host: str, server_port: int):
    url = 'http://%s:%d/' % (server_host, server_port)

    # waiting for server
    print('waiting for gorse ...')
    while True:
        status = get(url + 'status')
        if status['FeedbackCommit'] > 0:
            break
        time.sleep(1)

    # Load data
    data = pd.read_csv('~/.gorse/dataset/ml-100k/u1.test', sep='\t',
                       names=['user_id', 'item_id', 'rating', 'timestamp'])
    users = data.groupby('user_id')

    # Evaluate item pop
    prec_score, recall_score, ndcg_score = eval_item_pop(url, users)
    print('Precision@10 = %f, Recall@10 = %f, NDCG@10 = %f' % (prec_score, recall_score, ndcg_score))
    if prec_score - 0.19081 < -EPSILON:
        print('--- FAIL  precision is low')
        exit(1)
    if recall_score - 0.11584 < -EPSILON:
        print('--- FAIL  recall is low')
        exit(1)
    if ndcg_score - 0.21785 < -EPSILON:
        print('--- FAIL  NDCG is low')
        exit(1)

    # Evaluate SVD BPR
    prec_score, recall_score, ndcg_score = eval_svd_bpr(url, users)
    print('Precision@10 = %f, Recall@10 = %f, NDCG@10 = %f' % (prec_score, recall_score, ndcg_score))
    if prec_score - 0.32083 < -EPSILON:
        print('--- FAIL  precision is low')
        exit(1)
    if recall_score - 0.20906 < -EPSILON:
        print('--- FAIL  recall is low')
        exit(1)
    if ndcg_score - 0.37643 < -EPSILON:
        print('--- FAIL  NDCG is low')
        exit(1)


if __name__ == '__main__':

    # Print usage
    if len(sys.argv) < 3:
        print('%s [host] [port]' % sys.argv[0])
        exit(1)

    # Pass arguments
    host = sys.argv[1]
    port = int(sys.argv[2])

    # Test server
    test_server(host, port)
