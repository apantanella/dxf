# Security

## Threat model

The library processes DXF files that may come from untrusted sources. The main
risks are:

| Risk | Mitigation |
| --- | --- |
| Excessive memory usage | Configurable limits on entities, blocks and value size |
| Infinite loop or hang | No unbounded loop; limits produce `ErrLimit` |
| Panic on malformed input | Errors are returned, never panics; fuzz tests are mandatory |
| Allocation driven by declared counts | Counts from the file never drive allocations without verification |
| Unexpected file access | The core uses `io.Reader`/`io.Writer` and never opens files |
| Code execution | None, the format is data |

## Implementation principles

1. Never `panic` on input controlled by the file.
2. Every limit has a documented default and an option to change it.
3. The parser never opens files and never performs network requests.
4. Error messages report line, section and entity, without sensitive content.
5. Exported functions document their behavior on invalid input.

## Default limits

| Limit | Default value | Error |
| --- | --- | --- |
| Total entities | 1,000,000 | `ErrLimit` |
| Blocks | 100,000 | `ErrLimit` |
| Size of a single text value | 1 MiB | `ErrLimit` |
| Structure nesting | 64 | `ErrLimit` |

The values are indicative and will be confirmed by the benchmarks of Phase 5.

## Fuzzing

The scanner and the parser must have fuzz tests that verify:

- absence of panics;
- absence of unbounded allocations;
- termination within the limits.

```powershell
go test -fuzz=FuzzDecode -fuzztime=60s ./...
```

A corpus entry that causes a failure must be added to `testdata/` as a
regression case.

## Reporting

To report a vulnerability, open a private issue through the repository security
advisory mechanism, without publishing details in a public issue. Include:

- library version;
- minimal input file that reproduces the problem;
- expected and observed behavior;
- stack trace, if any.

Do not attach confidential project files: reduce the case to the minimum
necessary.

## Supported versions

Before `v1.0.0` only the latest published version receives security fixes.
