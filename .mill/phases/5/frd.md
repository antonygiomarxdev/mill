# FRD: Adapter interface completion

## User need
Mill needs a complete adapter interface to support multiple AI providers beyond CommandCode.

## Functional requirements
1. Adapter interface supports Dispatch, Resume, Capabilities
2. Claude adapter as second implementation
3. Provider auto-detection from environment

## Out of scope
- OpenCode adapter (deleted per ADR 0001)
- Multi-provider pooling

## Priority
Unblocks multi-provider support. Currently single-provider only.
