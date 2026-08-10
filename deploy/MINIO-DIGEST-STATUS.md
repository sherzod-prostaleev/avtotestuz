# MinIO image digest status

Assessed 2026-08-10 against Docker Hub's published `RELEASE.*` tags and
Trivy's HIGH/CRITICAL vulnerability database.  Raw discovery, pull, and scan
evidence is local-only under `.run/security-scans/`.

## Current production pins

| Image | Production RepoDigest | HIGH | CRITICAL |
| --- | --- | ---: | ---: |
| `minio/minio` | `sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e` | 76 | 6 |
| `minio/mc` | `sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727` | 34 | 2 |

## Published candidates tried

| Image | Latest ordinary `RELEASE.*` tag found | Pulled RepoDigest | Result |
| --- | --- | --- | --- |
| `minio/minio` | `RELEASE.2025-09-07T16-13-09Z` | `sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e` | Exact alias of the production pin; 76 HIGH, 6 CRITICAL |
| `minio/mc` | `RELEASE.2025-08-13T08-35-41Z` | `sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727` | Exact alias of the production pin; 34 HIGH, 2 CRITICAL |

`latest` was also pulled for both repositories and resolved to the same
respective production RepoDigest. Therefore no different, newer published
digest was found, let alone one with fewer HIGH/CRITICAL findings.

## Decision: BLOCKED — leave production unchanged

Do not change `deploy/docker-compose.prod.yml` or the matching backup drill
digest examples. Recheck when MinIO publishes a new ordinary release digest:
pull it by tag, record its Docker `RepoDigest`, and scan that immutable digest
before considering a pin update. A change is justified only with an evidenced
reduction in HIGH/CRITICAL findings (preferably zero).

Evidence files: `minio-hub-tags-page1.json`, `minio-mc-hub-tags-page1.json`,
`candidate-repodigests.txt`, and `minio-vulnerability-counts.jsonl`.
