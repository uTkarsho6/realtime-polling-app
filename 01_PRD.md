# Product Requirements Document (PRD)
## Real-Time Serverless Polling & Voting App

**Version:** 1.0
**Author:** [Your Name]
**Date:** [Insert Date]
**Project Type:** AWS Cloud Computing — Skills Demonstration Project

---

## 1. Purpose

This document defines the product goals, scope, and success criteria for a real-time
polling and voting web application built entirely on AWS serverless services. The
project is intended to demonstrate practical, hands-on AWS cloud computing skills
learned during training, within the constraints of the AWS Free Tier.

## 2. Problem Statement

Most beginner cloud projects use simple request/response (REST) patterns, where a
client must repeatedly ask the server "has anything changed?" (polling). This is
inefficient and does not reflect how many real-world systems (live scores, chat
apps, dashboards, auctions) actually work. There is a need for a small, well-scoped
project that demonstrates **event-driven, real-time architecture** on AWS, while
still being achievable by a single developer within free-tier limits.

## 3. Objectives

1. Design and deploy a working serverless backend on AWS using Lambda, API Gateway,
   and DynamoDB.
2. Implement real-time, bidirectional communication using WebSockets (as opposed to
   traditional polling).
3. Demonstrate understanding of stateful connection management in an otherwise
   stateless (serverless) environment.
4. Solve a real concurrency problem (simultaneous votes) using atomic database
   operations.
5. Deploy and document the entire system using Infrastructure as Code (AWS SAM).
6. Stay entirely within AWS Free Tier usage limits.

## 4. Target Users / Use Case

- **Primary user:** Anyone who wants to quickly create a poll and share it with a
  group (e.g., a small team deciding on lunch, a classroom quiz, a community vote).
- **Demonstration audience:** Evaluators/reviewers assessing AWS cloud computing
  competency (training rubric).

## 5. Scope

### In Scope
- Creating a poll with a question and 2–5 options
- Sharing a poll via a unique link/ID
- Casting a vote (one vote per connected session, enforced at application level)
- Live, real-time update of vote counts to all connected viewers of a poll
- Basic static frontend to create polls, vote, and view live results
- Deployment via AWS SAM (Infrastructure as Code)

### Out of Scope (for v1)
- User authentication / login system
- Preventing duplicate votes across devices (IP-based or account-based)
- Poll expiry / scheduling
- Analytics dashboard beyond raw vote counts
- Multi-region deployment

## 6. User Stories

| ID | As a... | I want to... | So that... |
|----|---------|---------------|------------|
| US-1 | Poll creator | Create a poll with a question and multiple options | I can gather opinions from a group |
| US-2 | Poll creator | Get a shareable link after creating a poll | Others can find and vote on it |
| US-3 | Voter | Open a poll link and cast a vote | My opinion is counted |
| US-4 | Voter | See results update live without refreshing | I can watch the outcome in real time |
| US-5 | Voter | See results even if I don't vote | I can just observe the poll |

## 7. Functional Requirements

- FR-1: System shall allow creation of a poll with a question and 2–5 options.
- FR-2: System shall generate a unique poll ID on creation.
- FR-3: System shall allow a user to submit exactly one vote per poll per session.
- FR-4: System shall update vote counts atomically to avoid lost updates under
  concurrent voting.
- FR-5: System shall push updated results to all clients currently viewing that poll,
  in real time, via WebSocket.
- FR-6: System shall clean up stale/disconnected WebSocket connections.

## 8. Non-Functional Requirements

- NFR-1: All AWS resource usage must remain within Free Tier limits.
- NFR-2: The system must be serverless (no always-on EC2 instances).
- NFR-3: Infrastructure must be defined as code (AWS SAM) for reproducibility.
- NFR-4: The system should handle at least 10 concurrent connections per poll
  without errors (sufficient for demo purposes).

## 9. Success Criteria

| Criteria | Definition of Done |
|---|---|
| Working demo | A poll can be created, voted on, and results update live across 2+ browser tabs |
| Real concurrency handling | Vote counts remain accurate when votes are submitted simultaneously |
| Free-tier compliance | AWS Billing dashboard shows $0 charges after testing |
| Documentation | PRD, TRD, architecture diagram, and task report completed |
| Code quality | Backend code (Go) is version-controlled on GitHub with a clear README |

## 10. Assumptions & Constraints

- Developer has an active AWS account with Free Tier eligibility.
- Project will be built and tested in a single AWS region.
- Traffic volume will be low (demo/portfolio scale), well under Free Tier caps.
- Development language for Lambda functions: **Go**.

## 11. Timeline (Phased Approach)

| Phase | Deliverable |
|---|---|
| Phase 1 | REST API: create poll, submit vote, fetch results (no real-time yet) |
| Phase 2 | WebSocket API: live connection management and result broadcasting |
| Phase 3 | Frontend + hosting on S3 |
| Phase 4 | Documentation, diagrams, and final report |
