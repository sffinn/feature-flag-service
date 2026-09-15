# Feature Flag Service

## 1. Summary
Production ready REST API service that stores feature flags, manages flag states globally and per-user, and evaluates feature availability for specific users while utilizing chaching for performance.

## 2. Goals
- Creation and storage of feature flags
- Managing flags globally or for a specific user
- Enpoint which evaluates if a flag is enabled for a given user
- Return proper HTTP status codes

## 3. Design

### 3.1 High-Level Arcitecture
```mermaid
flowchart TD
    Client[Client]
    AppPlatform[DigitalOcean App Platform]
    API[Go API Service]
    Redis[Redis Cache]
    Postgres[Postgres]

    Client -->|HTTP| AppPlatform
    AppPlatform --> API
    API -->|cache-aside read/write| Redis
    API -->|source of truth| Postgres
    Redis -->|cache miss| Postgres
```

### 3.2 Key Interfaces / Contracts
```
POST /v1/flag
Request:
{
    "user": string,
    "flagname": string,
    enabled: bool
}
Response:
{
    "flagname": string
}
```

```
GET /v1/flag
Request:
{
    "user": string,
    "flagname": string
}
Response:
{
    "flagname": string,
    "enabled": bool
}
```

```
GET /health
Response:
"healthy"

```go
type UserFlag struct {
    username string
    flagname string
    enabled bool
}
```

```go
type GlobalFlag struct {
    flagname string
    enabled bool
}
```

### 5.4 Internal Comonents
```mermaid
flowchart LR
    subgraph httpLayer [HTTP Layer]
        Health[GET /health]
        Create[POST /v1/flag]
        Evaluate[GET /v1/flag]
    end

    subgraph domain [Domain Layer]
        FlagService[flag.Service]
        Validate[ValidateFlagName]
    end

    subgraph persistence [Persistence]
        RedisCache[Redis Cache]
        PGStore[Postgres Store]
    end

    subgraph tables [Postgres Tables]
        GlobalFlags[global_flags]
        UserFlags[user_flags]
    end

    Health --> FlagService
    Create --> FlagService
    Evaluate --> FlagService
    FlagService --> Validate
    FlagService --> RedisCache
    FlagService --> PGStore
    PGStore --> GlobalFlags
    PGStore --> UserFlags
```

### 5.5 Data Flow / Sequence
**Create Flag**
```mermaid
sequenceDiagram
    participant Client
    participant API as Go API
    participant Service as flag.Service
    participant PG as Postgres
    participant Redis

    Client->>API: POST /v1/flag
    Note over Client,API: user empty = global<br/>user set = override

    API->>Service: Create(user, flagname, enabled)
    Service->>Service: ValidateFlagName

    alt invalid flag name
        Service-->>API: ErrInvalidFlagName
        API-->>Client: 400 Bad Request
    else global flag
        Service->>PG: INSERT global_flags
        alt already exists
            PG-->>Service: unique violation
            Service-->>API: ErrFlagExists
            API-->>Client: 409 Conflict
        else success
            Service->>Redis: DELETE flag:global:{flagname}
            Service-->>API: flagname
            API-->>Client: 201 Created
        end
    else user override
        Service->>PG: INSERT user_flags
        alt already exists
            PG-->>Service: unique violation
            Service-->>API: ErrFlagExists
            API-->>Client: 409 Conflict
        else success
            Service->>Redis: DELETE flag:user:{user}:{flagname}
            Service-->>API: flagname
            API-->>Client: 201 Created
        end
    end
```

**Update Flag**
```mermaid
sequenceDiagram
    participant Client
    participant API as Go API
    participant Service as flag.Service
    participant PG as Postgres
    participant Redis

    Client->>API: POST /v1/flag
    Note over Client,API: user empty = global<br/>user set = override

    API->>Service: Create(user, flagname, enabled)
    Service->>Service: ValidateFlagName

    alt invalid flag name
        Service-->>API: ErrInvalidFlagName
        API-->>Client: 400 Bad Request
    else global flag
        Service->>PG: INSERT global_flags
        else success
            Service->>Redis: DELETE flag:global:{flagname}
            Service-->>API: flagname
            API-->>Client: 200 OK
        end
    else user override
        Service->>PG: INSERT user_flags
        else success
            Service->>Redis: DELETE flag:user:{user}:{flagname}
            Service-->>API: flagname
            API-->>Client: 200 OK
        end
    end
```

**Evaluate flag**
```mermaid
sequenceDiagram
    participant Client
    participant API as Go API
    participant Service as flag.Service
    participant Redis
    participant PG as Postgres

    Client->>API: GET /v1/flag?user=&flagname=
    API->>Service: Evaluate(user, flagname)
    Service->>Service: ValidateFlagName

    alt user provided
        Service->>Redis: GET flag:user:{user}:{flagname}
        alt cache hit
            Redis-->>Service: enabled
            Service-->>API: flagname, enabled
            API-->>Client: 200 OK
        else cache miss
            Service->>PG: SELECT user_flags
            alt user override found
                PG-->>Service: enabled
                Service->>Redis: SET key TTL 60s
                Service-->>API: flagname, enabled
                API-->>Client: 200 OK
            end
        end
    end

    Service->>Redis: GET flag:global:{flagname}
    alt cache hit
        Redis-->>Service: enabled
        Service-->>API: flagname, enabled
        API-->>Client: 200 OK
    else cache miss
        Service->>PG: SELECT global_flags
        alt global flag found
            PG-->>Service: enabled
            Service->>Redis: SET key TTL 60s
            Service-->>API: flagname, enabled
            API-->>Client: 200 OK
        else not found
            Service-->>API: ErrFlagNotFound
            API-->>Client: 404 Not Found
        end
    end
```

### 5.6 Error Handling & Edge Cases
**Post /v1/flag**
HTTP 400 Invalid Flag Name
HTTP 409 Flag Exists

**PUT /v1/flag**
HTTP 400 Invalid Flag Name

**Get /v1/flag**
HTTP 404 Flag Not Found

