# Happy Path

These diagrams describe the successful path from authentication through cleanup. Each section isolates one stage so the rendered graph stays readable.

## (1)–(3): Authentication and job creation

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "background": "#111111",
    "primaryColor": "#171717",
    "primaryTextColor": "#f5f5f5",
    "primaryBorderColor": "#38bdf8",
    "lineColor": "#d1d5db",
    "fontSize": "18px"
  },
  "flowchart": {
    "nodeSpacing": 100,
    "rankSpacing": 160,
    "curve": "basis"
  }
}}%%

flowchart LR
    C[Client]
    API[Go HTTPS API]
    DB[(PostgreSQL)]

    C -->|"(1) Login and send JWT"| API
    API -->|"(2) Verify identity and create job"| DB
    API -->|"(3) Return job ID and upload URL"| C
```

## (4)–(11): Upload and media inspection

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "background": "#111111",
    "primaryColor": "#171717",
    "primaryTextColor": "#f5f5f5",
    "primaryBorderColor": "#38bdf8",
    "lineColor": "#d1d5db",
    "fontSize": "18px"
  },
  "flowchart": {
    "nodeSpacing": 100,
    "rankSpacing": 160,
    "curve": "basis"
  }
}}%%

flowchart LR
    C[Client]
    API[Go HTTPS API]
    COORD[Coordinator]
    S[(Object Storage)]
    F[FFprobe]
    DB[(PostgreSQL)]

    C -->|"(4) Upload video"| S
    C -->|"(5) Finalize upload with checksum"| API
    API -->|"(6) Read job and object details"| DB
    API -->|"(7) Verify uploaded object"| S
    API -->|"(8) Ask Coordinator to inspect"| COORD
    COORD -->|"(9) Read video metadata"| F
    F -->|"(10) Return media information"| COORD
    COORD -->|"(11) Save metadata and create tasks"| DB
```

## (12)–(23): Worker processing and validation

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "background": "#111111",
    "primaryColor": "#171717",
    "primaryTextColor": "#f5f5f5",
    "primaryBorderColor": "#38bdf8",
    "lineColor": "#d1d5db",
    "fontSize": "18px"
  },
  "flowchart": {
    "nodeSpacing": 100,
    "rankSpacing": 160,
    "curve": "basis"
  }
}}%%

flowchart LR
    W[Contributor Worker]
    COORD[Coordinator]
    DB[(PostgreSQL)]
    S[(Object Storage)]
    F[FFmpeg]
    V[Result Validator]

    W -->|"(12) Poll for task"| COORD
    COORD -->|"(13) Create attempt and lease"| DB
    COORD -->|"(14) Return task and scoped URLs"| W
    W -->|"(15) Download input"| S
    W -->|"(16) Run encoding"| F
    W -->|"(17) Send heartbeat"| COORD
    W -->|"(18) Upload output"| S
    W -->|"(19) Submit result"| COORD
    COORD -->|"(20) Request validation"| V
    V -->|"(21) Read and validate output"| S
    V -->|"(22) Return valid result"| COORD
    COORD -->|"(23) Accept result"| DB
```

## (24)–(37): Assembly, credits, download, and cleanup

```mermaid
%%{init: {
  "theme": "base",
  "themeVariables": {
    "background": "#111111",
    "primaryColor": "#171717",
    "primaryTextColor": "#f5f5f5",
    "primaryBorderColor": "#38bdf8",
    "lineColor": "#d1d5db",
    "fontSize": "18px"
  },
  "flowchart": {
    "nodeSpacing": 100,
    "rankSpacing": 160,
    "curve": "basis"
  }
}}%%

flowchart LR
    COORD[Coordinator]
    DB[(PostgreSQL)]
    A[Final Assembler]
    S[(Object Storage)]
    V[Result Validator]
    L[Credit Ledger]
    C[Client]
    CL[Cleanup Worker]

    COORD -->|"(24) Confirm all tasks accepted"| DB
    COORD -->|"(25) Start final assembly"| A
    A -->|"(26) Read accepted outputs"| S
    A -->|"(27) Write final video"| S
    A -->|"(28) Request final validation"| V
    V -->|"(29) Validate final video"| S
    V -->|"(30) Return successful validation"| COORD
    COORD -->|"(31) Mark job completed"| DB
    COORD -->|"(32) Record worker reward"| L
    L -->|"(33) Save ledger entry"| DB
    C -->|"(34) Request completed job"| COORD
    COORD -->|"(35) Return download URL"| C
    C -->|"(36) Download final video"| S
    CL -->|"(37) Delete expired temporary objects"| S
```
