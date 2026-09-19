# Project Overview
## Real-Time Serverless Polling & Voting App

**Related documents:** 01_PRD.md, 02_TRD.md, 03_Task_Report.md

This document explains, in plain language, how the whole system fits together —
useful as a quick reference for your presentation and for anyone reading your
GitHub README.

---

## 1. The Idea in One Sentence

A user creates a poll, shares the link, and everyone who opens it sees vote counts
update **live** — no page refresh — because the backend pushes updates to
connected browsers over a WebSocket instead of making them ask for updates.

## 2. The Two Paths Through the System

The system has two separate "lanes" that both talk to the same DynamoDB database:

- **The Write Lane (REST API):** used to create a poll and to submit a vote.
- **The Live Lane (WebSocket API):** used to keep a connection open and push
  live results to every browser watching a poll.

## 3. Architecture Diagram

```mermaid
flowchart TB
    subgraph Client["Client (Browser)"]
        UI[Poll UI: create / vote / live results]
    end

    subgraph REST["REST API — Write Path"]
        APIGW_REST[API Gateway - REST API]
        L_Create[Lambda: CreatePoll]
        L_Vote[Lambda: SubmitVote]
        L_Get[Lambda: GetPoll]
    end

    subgraph WS["WebSocket API — Real-Time Path"]
        APIGW_WS[API Gateway - WebSocket API]
        L_Connect[Lambda: OnConnect]
        L_Disconnect[Lambda: OnDisconnect]
        L_Broadcast[Lambda: Broadcast - called by SubmitVote]
    end

    subgraph Data["Data Layer"]
        DDB_Polls[(DynamoDB: Polls Table)]
        DDB_Conn[(DynamoDB: Connections Table)]
    end

    subgraph Hosting["Static Frontend Hosting"]
        S3[S3 Static Website]
        CF[CloudFront - optional]
    end

    UI -- HTTPS requests --> APIGW_REST
    APIGW_REST --> L_Create --> DDB_Polls
    APIGW_REST --> L_Get --> DDB_Polls
    APIGW_REST --> L_Vote
    L_Vote -- atomic UpdateItem --> DDB_Polls
    L_Vote -- triggers --> L_Broadcast
    L_Broadcast -- query connections by PollId --> DDB_Conn
    L_Broadcast -- PostToConnection --> APIGW_WS
    APIGW_WS -- pushes live update --> UI

    UI -- WebSocket connect --> APIGW_WS
    APIGW_WS --> L_Connect --> DDB_Conn
    APIGW_WS --> L_Disconnect --> DDB_Conn

    CF --> S3
    S3 -- serves --> UI
```

## 4. Voting Flow (Sequence / Flowchart)

This shows what happens, step by step, the moment someone clicks "Vote":

```mermaid
sequenceDiagram
    participant U1 as User 1 (votes)
    participant U2 as User 2 (just watching)
    participant WS as WebSocket API
    participant Rest as REST API
    participant Vote as Lambda: SubmitVote
    participant DB as DynamoDB: Polls
    participant Conn as DynamoDB: Connections
    participant BC as Lambda: Broadcast

    Note over U1,U2: Both already connected via WebSocket ($connect ran earlier)

    U1->>Rest: POST /polls/{id}/vote {option: "Python"}
    Rest->>Vote: invoke SubmitVote
    Vote->>DB: UpdateItem (ADD count 1) — atomic, race-condition safe
    DB-->>Vote: updated vote counts
    Vote->>BC: trigger broadcast(pollId, updatedResults)
    BC->>Conn: query all ConnectionIds where PollId = id
    Conn-->>BC: [connId_U1, connId_U2]
    BC->>WS: PostToConnection(connId_U1, updatedResults)
    BC->>WS: PostToConnection(connId_U2, updatedResults)
    WS-->>U1: live update pushed
    WS-->>U2: live update pushed
    Note over U1,U2: Both screens update instantly, no refresh needed
```

## 5. Connection Lifecycle Flowchart

```mermaid
flowchart LR
    A[Browser opens poll page] --> B[Opens WebSocket connection]
    B --> C{API Gateway: $connect route}
    C --> D[Lambda: OnConnect]
    D --> E[(Save ConnectionId + PollId\nin Connections table)]
    E --> F[Connection active - receives live updates]
    F --> G{Browser closes tab\nor loses connection}
    G --> H{API Gateway: $disconnect route}
    H --> I[Lambda: OnDisconnect]
    I --> J[(Delete ConnectionId\nfrom Connections table)]
```

## 6. Component Summary

| Component | Plain-English role |
|---|---|
| **API Gateway (REST)** | The "front door" for creating polls and casting votes |
| **API Gateway (WebSocket)** | Keeps a live, open line to every browser watching a poll |
| **Lambda functions** | Small pieces of Go code that run only when triggered — no server sits idle |
| **DynamoDB: Polls table** | Stores the question, options, and current vote counts |
| **DynamoDB: Connections table** | Tracks which browsers are currently "listening" to which poll |
| **S3 (+ CloudFront)** | Hosts the simple frontend page that users actually see |

## 7. Why This Design Solves a Real Problem

- **Without WebSockets:** every browser would have to keep asking "any new
  votes?" every few seconds (polling) — wasteful and laggy.
- **With WebSockets:** the server pushes the update the instant a vote is cast —
  instantaneous and efficient.
- **Without atomic counters:** two people voting at the exact same moment could
  overwrite each other's vote (read old count → both add 1 → write same new
  count → one vote lost).
- **With atomic `UpdateItem ADD`:** DynamoDB guarantees the increment happens
  safely even under concurrent requests — no votes lost.

## 8. Free Tier Footprint (Quick Reference)

| Service | Free tier allowance used |
|---|---|
| Lambda | A handful of invocations per test session, against 1M/month free |
| DynamoDB | A few KB of data, against 25GB free storage |
| API Gateway (REST + WebSocket) | A handful of calls/messages, against 1M/month free each |
| S3 | A few KB of static files, against 5GB free storage |

This project, even with heavy manual testing, stays far below Free Tier limits.
