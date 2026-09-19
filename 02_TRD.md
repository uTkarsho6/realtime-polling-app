# Technical Requirements Document (TRD)
## Real-Time Serverless Polling & Voting App

**Version:** 1.0
**Related document:** 01_PRD.md
**Date:** [Insert Date]

---

## 1. Technology Stack

| Layer | Technology | Notes |
|---|---|---|
| Compute | AWS Lambda (Go, `provided.al2023` runtime via `aws-lambda-go`) | Fast cold starts, efficient compute usage |
| REST API | Amazon API Gateway (REST API) | Create poll, submit vote, fetch results |
| Real-time API | Amazon API Gateway (WebSocket API) | Live connection + broadcast |
| Database | Amazon DynamoDB | Polls table + Connections table |
| SDK | `aws-sdk-go-v2` | DynamoDB + API Gateway Management API |
| Frontend | HTML / CSS / vanilla JavaScript | Native `WebSocket` browser API |
| Hosting | Amazon S3 (static website hosting) | Optional: CloudFront in front |
| IaC / Deployment | AWS SAM (Serverless Application Model) | `template.yaml` defines all resources |
| Monitoring | Amazon CloudWatch | Logs + a billing alarm |
| Version control | Git + GitHub | Public repo for portfolio |

## 2. System Architecture (Summary)

Two independent API Gateway APIs share a common DynamoDB backend:

- **REST API** → handles poll creation and vote submission (write path).
- **WebSocket API** → handles live connections and pushes result updates
  (real-time path).

See `03_Project_Overview.md` for the full architecture diagram and flowchart.

## 3. Data Model (DynamoDB)

### Table 1: `Polls`

| Attribute | Type | Description |
|---|---|---|
| `PollId` (PK) | String | Unique poll identifier (UUID) |
| `Question` | String | The poll question |
| `Options` | Map | `{ "OptionA": {count: N}, "OptionB": {count: N}, ... }` |
| `CreatedAt` | String (ISO 8601) | Creation timestamp |

Vote counts are updated using DynamoDB's `UpdateItem` with an `ADD` expression
(atomic counter), which prevents lost updates when multiple votes arrive at the
same time.

### Table 2: `Connections`

| Attribute | Type | Description |
|---|---|---|
| `ConnectionId` (PK) | String | WebSocket connection ID (from API Gateway) |
| `PollId` (GSI) | String | Which poll this connection is watching |
| `ConnectedAt` | String (ISO 8601) | Connection timestamp |

A Global Secondary Index (GSI) on `PollId` allows the broadcast function to quickly
look up all connections subscribed to a given poll.

## 4. API Design

### REST API (write path)

| Method | Route | Purpose |
|---|---|---|
| POST | `/polls` | Create a new poll → returns `PollId` |
| POST | `/polls/{pollId}/vote` | Submit a vote for an option |
| GET | `/polls/{pollId}` | Fetch current poll state (question, options, counts) |

### WebSocket API (real-time path)

| Route | Trigger | Purpose |
|---|---|---|
| `$connect` | Client opens WebSocket connection | Store `ConnectionId` + `PollId` in `Connections` table |
| `$disconnect` | Client closes/loses connection | Remove `ConnectionId` from `Connections` table |
| `$default` | Any other message | Handle ping/keepalive or ignore |

Broadcast is not a client-invoked route — it's triggered server-side from the
`vote` Lambda after a successful `UpdateItem`, using the **API Gateway Management
API** (`PostToConnection`) to push the updated results to every connection ID
associated with that poll.

## 5. Lambda Functions

| Function | Trigger | Responsibility |
|---|---|---|
| `CreatePoll` | REST POST `/polls` | Validate input, write new poll to `Polls` table |
| `SubmitVote` | REST POST `/polls/{pollId}/vote` | Atomically increment vote count, then invoke broadcast logic |
| `GetPoll` | REST GET `/polls/{pollId}` | Read and return current poll state |
| `OnConnect` | WebSocket `$connect` | Save connection ID + poll ID to `Connections` table |
| `OnDisconnect` | WebSocket `$disconnect` | Delete connection ID from `Connections` table |
| `Broadcast` | Called internally by `SubmitVote` | Query all connections for a poll, push updated results to each via `PostToConnection`; remove any connection IDs that return a "gone" (410) error |

## 6. Non-Functional / Technical Requirements

- **Concurrency safety:** All vote increments use DynamoDB atomic `ADD` operations,
  not read-then-write logic, to avoid race conditions.
- **Stale connection cleanup:** The `Broadcast` function must catch `GoneException`
  (HTTP 410) responses from `PostToConnection` and delete the corresponding
  connection record — otherwise dead connections accumulate.
- **Cost control:** DynamoDB table billing mode set to Provisioned (5 RCU / 5 WCU)
  to stay predictably within Free Tier, or On-Demand with a CloudWatch billing
  alarm as a safety net.
- **IaC:** All resources (Lambda functions, both API Gateway APIs, DynamoDB tables,
  IAM roles) defined in a single `template.yaml` using AWS SAM.
- **Security:** IAM roles scoped with least-privilege — each Lambda only gets the
  specific DynamoDB/API Gateway permissions it needs.
- **Region:** Single AWS region (e.g., `ap-south-1` or `us-east-1`) to keep Free
  Tier tracking simple.

## 7. Testing Strategy

| Test | Method |
|---|---|
| Unit tests | Go's built-in `testing` package for handler logic (mocking DynamoDB via interfaces) |
| Local integration | `sam local start-api` and `sam local invoke` before deploying |
| Manual concurrency test | Open 3–4 browser tabs, vote simultaneously, verify final count matches number of votes cast |
| Manual real-time test | Vote in one tab, confirm result updates live in the other tabs without refresh |
| Cost check | Review AWS Billing dashboard after testing session |

## 8. Deployment Process (Overview)

1. `sam build` — compiles Go binaries and packages the app.
2. `sam deploy --guided` — first-time deployment, creates a CloudFormation stack.
3. Subsequent changes: `sam build && sam deploy`.
4. Frontend deployed separately via `aws s3 sync` to the static hosting bucket.

Full step-by-step execution is documented in `03_Task_Report.md`.
