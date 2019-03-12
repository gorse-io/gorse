---
# Feel free to add content and custom Front Matter to this file.
# To modify the layout, see https://jekyllrb.com/docs/themes/#overriding-theme-defaults

layout: splash
header:
  overlay_color: "#555"
  overlay_filter: "0.5"
  actions:
    - label: "Download"
      url: "https://github.com/zhenghaoz/gorse"
excerpt: "A Transparent Recommender System Engine over SQL Database based on Collaborative Filtering written in Go."

feature_row:
  - image_path: assets/images/unsplash-gallery-image-1-th.jpg
    alt: "placeholder image 1"
    title: "Collaborative Filtering"
    excerpt: "**Collaborative filtering** models are used to generate recommendation. Models includes matrix factorization, k nearest neighbor, slope one and co-clustering."
  - image_path: /assets/images/unsplash-gallery-image-2-th.jpg
    alt: "placeholder image 2"
    title: "SQL Database"
    excerpt: "Use **SQL Database** as storage. Data are loaded and fed to the model automatically. Recommendations are generated and cached in database."
  - image_path: /assets/images/unsplash-gallery-image-3-th.jpg
    title: "Written in Go"
    excerpt: "The engine is **written in Go**, cooperates with multithreading and SIMD. It means high performance and easy to deploy with a standalone binary."
---
{% include feature_row %}
