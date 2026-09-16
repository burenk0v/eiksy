# AI domain

`Provider` is the runtime boundary for AI backends. Implementations may use local or remote OpenAI-compatible endpoints, but provider credentials and transport details must remain outside domain request/response objects.
