---
# Feel free to add content and custom Front Matter to this file.
# To modify the layout, see https://jekyllrb.com/docs/themes/#overriding-theme-defaults

layout: splash
header:
  overlay_color: "#555"
  overlay_filter: "0.0"
  overlay_image: https://img.sine-x.com/home.png
  actions:
    - label: "Download"
      url: "https://github.com/zhenghaoz/gorse"
excerpt: "A Transparent Recommender System Engine over SQL Database based on Collaborative Filtering written in Go."

feature_row:
  - image_path: https://img.sine-x.com/feature-crowd.png
    title: "Collaborative filtering"
    excerpt: "**Collaborative filtering** models are used to generate recommendation. Models includes matrix factorization, k nearest neighbor, slope one and co-clustering."
  - image_path: https://img.sine-x.com/feature-database.png
    title: "Over SQL Database"
    excerpt: "Use **SQL Database** as storage. Data are loaded and fed to the model automatically. Recommendations are generated and cached in database."
  - image_path: https://img.sine-x.com/feature-package.png
    title: "Out of the box"
    excerpt: "It's easy to deploy with a standalone binary. Provides CLI tools and RPC interfaces. Recommender system could be built without coding."
---
{% include feature_row %}
