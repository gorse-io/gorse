# Gorse Open-source Recommender System Engine

<img width=160 src="assets/gorse.png"/>

![](https://img.shields.io/github/go-mod/go-version/zhenghaoz/gorse)
[![test](https://github.com/gorse-io/gorse/actions/workflows/build_test.yml/badge.svg)](https://github.com/gorse-io/gorse/actions/workflows/build_test.yml)
[![codecov](https://codecov.io/gh/gorse-io/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/gorse-io/gorse)
[![Discord](https://img.shields.io/discord/830635934210588743)](https://discord.gg/x6gAtNNkAE)
[![Twitter Follow](https://img.shields.io/twitter/follow/gorse_io?label=Follow&style=social)](https://twitter.com/gorse_io)
[![Gurubase](https://img.shields.io/badge/Gurubase-Ask%20Gorse%20Guru-006BFF)](https://gurubase.io/g/gorse)

Gorse is an AI powered open-source recommender system written in Go. Gorse aims to be a universal open-source recommender system that can be quickly integrated into a wide variety of online services. By importing items, users, and interaction data into Gorse, the system will automatically train models to generate recommendations for each user. Project features are as follows.

![](https://github.com/gorse-io/docs/blob/main/src/img/dashboard/recflow.png?raw=true)

- **Multi-source:** Recommend items from latest, user-to-user, item-to-item, collaborative filtering and etc.
- **Multimodal:** Support multimodal content (text, image, videos, etc.) via embedding.
- **AI-powered:** Support both classical recommenders and LLM-based recommenders.
- **GUI Dashboard:** Provide GUI dashboard for recommendation pipeline editing, system monitoring, and data management.
- **RESTful APIs:** Expose RESTful APIs for data CRUD and recommendation requests.

## Quick Start

The playground mode has been prepared for beginners. Just set up a recommender system for GitHub repositories by the following commands.

```bash
docker run -p 8088:8088 zhenghaoz/gorse-in-one --playground
```

The playground mode will download data from [GitRec](https://gitrec.gorse.io/) and import it into Gorse. The dashboard is available at `http://localhost:8088`.

![](https://github.com/gorse-io/docs/blob/main/src/img/dashboard/overview.png?raw=true)

After the "Generate item-to-item recommendation" task is completed on the "Tasks" page, try to insert several feedbacks into Gorse. Suppose Bob is a developer who interested in LLM related repositories. We insert his star feedback to Gorse.

```bash
read -d '' JSON << EOF
[
    { \"FeedbackType\": \"star\", \"UserId\": \"bob\", \"ItemId\": \"ollama:ollama\", \"Value\": 1.0, \"Timestamp\": \"2022-02-24\" },
    { \"FeedbackType\": \"star\", \"UserId\": \"bob\", \"ItemId\": \"huggingface:transformers\", \"Value\": 1.0, \"Timestamp\": \"2022-02-25\" },
    { \"FeedbackType\": \"star\", \"UserId\": \"bob\", \"ItemId\": \"rasbt:llms-from-scratch\", \"Value\": 1.0, \"Timestamp\": \"2022-02-26\" },
    { \"FeedbackType\": \"star\", \"UserId\": \"bob\", \"ItemId\": \"vllm-project:vllm\", \"Value\": 1.0, \"Timestamp\": \"2022-02-27\" },
    { \"FeedbackType\": \"star\", \"UserId\": \"bob\", \"ItemId\": \"hiyouga:llama-factory\", \"Value\": 1.0, \"Timestamp\": \"2022-02-28\" }
]
EOF

curl -X POST http://127.0.0.1:8088/api/feedback \
   -H 'Content-Type: application/json' \
   -d "$JSON"
```

Then, fetch 10 recommended items from Gorse. We can find that LLM-related repositories are recommended for Bob.

```bash
curl http://127.0.0.1:8088/api/recommend/bob?n=10
```

For more information：

- Read [official documents](https://gorse.io/docs/)
- Visit [playground](https://play.gorse.io/) of Gorse dashboard
- Explore [live demo](https://gitrec.gorse.io/), a recommender system for GitHub repositories
- Discuss on [Discord](https://discord.gg/x6gAtNNkAE) or [GitHub Discussion](https://github.com/gorse-io/gorse/discussions)

## Architecture

Gorse is a single-node training and distributed prediction recommender system. Gorse stores data in MySQL, MongoDB, Postgres, or ClickHouse, with intermediate results cached in Redis, MySQL, MongoDB and Postgres.

1. The cluster consists of a master node, multiple worker nodes, and server nodes.
1. The master node is responsible for model training, non-personalized recommendation, configuration management, and membership management.
1. The server node is responsible for exposing the RESTful APIs and online real-time recommendations.
1. Worker nodes are responsible for offline recommendations for each user.

In addition, the administrator can perform system monitoring, data import and export, and system status checking via the dashboard on the master node.

<img width=520 src="https://github.com/gorse-io/docs/blob/main/src/img/cluster.drawio.svg?raw=true"/>

## Contributors

<a href="https://github.com/gorse-io/gorse/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=zhenghaoz/gorse" />
</a>

Any contribution is appreciated: report a bug, give advice or create a pull request. Read [CONTRIBUTING.md](CONTRIBUTING.md) for more information.

## Acknowledgments

`gorse` is inspired by the following projects:

- [Guibing Guo's librec](https://github.com/guoguibing/librec)
- [Nicolas Hug's Surprise](https://github.com/NicolasHug/Surprise)
- [Golang Samples's gopher-vector](https://github.com/golang-samples/gopher-vector)
