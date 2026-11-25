# Storage adapters

## 1. Core must be storage agnostic

The core should define the minimum operations needed to load and store bucket entries and their metadata.

It must not import Redis clients, SQL drivers, ORMs, or cloud SDKs.

## 2. Adapter responsibilities

An adapter is responsible for:
- mapping the conceptual identity (`Namespace + ReportID + ScopeKey + Bucket + QueryVersion`) into safe storage keys without delimiter collision risks;
- payload serialization and deserialization;
- atomic metadata/payload writes where possible;
- conditional writes or compare-and-swap if supported;
- TTL implementation;
- connection and retry configuration;
- documenting consistency guarantees and limitations.

The core is responsible for:
- validating request identity;
- planning;
- validity policy;
- execution orchestration;
- deciding whether an entry can be reused.

## 3. In-memory adapter

A small in-memory implementation is useful for:
- unit tests;
- examples;
- local development;
- validating the public interface.

It is not a substitute for a durable distributed cache in production.

## 4. Redis adapter

Redis may be a useful optional adapter because it supports fast key-value access and atomic operations.

Do not make Redis mandatory. Keep the adapter separate, for example:

```text
github.com/Khan9Tran/reportcache
github.com/Khan9Tran/reportcache/adapters/redis
```

Before implementing Redis support, decide:
- key schema representing `Namespace + ReportID + ScopeKey + Bucket + QueryVersion`;
- payload encoding;
- metadata layout;
- TTL semantics;
- atomic write strategy;
- generation/CAS strategy;
- lock behavior, if any;
- key prefix and versioning;
- behavior during Redis outage.

## 5. Database adapter

A database-backed cache may be useful for durable report snapshots, but should not be part of the first core milestone unless a concrete use case requires it.

## 6. Adapter compatibility

Each adapter should run the same contract test suite:
- get missing key;
- set and get entry;
- scope isolation (different `ScopeKey` or `Namespace` values never collide);
- preserve metadata;
- expired entry behavior;
- concurrent writes;
- generation conflict;
- serialization error;
- context cancellation.
