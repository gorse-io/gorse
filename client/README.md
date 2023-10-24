# Gorse Client

Go SDK for Gorse recommender system.

> ⚠️⚠️⚠️ This SDK is unstable currently. APIs might be changed in later versions.

## Install

```bash
go get github.com/Neura-Studios/gorse/client@master
```

## Usage

```go
import "github.com/Neura-Studios/gorse/client"

gorse := client.NewGorseClient("http://127.0.0.1:8087", "api_key")

gorse.InsertFeedback([]client.Feedback{
    {FeedbackType: "star", UserId: "bob", ItemId: "vuejs:vue", Timestamp: "2022-02-24"},
    {FeedbackType: "star", UserId: "bob", ItemId: "d3:d3", Timestamp: "2022-02-25"},
    {FeedbackType: "star", UserId: "bob", ItemId: "dogfalo:materialize", Timestamp: "2022-02-26"},
    {FeedbackType: "star", UserId: "bob", ItemId: "mozilla:pdf.js", Timestamp: "2022-02-27"},
    {FeedbackType: "star", UserId: "bob", ItemId: "moment:moment", Timestamp: "2022-02-28"},
})

gorse.GetRecommend("bob", "", 10)
```

## Test

In the root directory of Gorse source:

```bash
# Setup Gorse
docker-compose up -d

# Test
go test -tags='integrate_test' ./client/
```
