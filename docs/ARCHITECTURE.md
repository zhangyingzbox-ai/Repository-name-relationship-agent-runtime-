# Architecture

## Runtime Loop

`AgentRuntime.Chat` is the single orchestration point:

1. Validate user input and create a fallback user id when needed.
2. Load long-term memory through `MemoryTool`.
3. Extract structured facts through `ExtractionTool`.
4. Update `UserProfile`, `MemoryHistory`, `Conflicts`, and `RelationshipState`.
5. Persist memory.
6. Generate a response from the current message and structured memory.

Every step appends a `TraceStep`, so the caller can inspect what happened.

## Data Model

- `UserProfile`: current long-term memory for one user.
- `BasicInfo`: name, age, occupation, city, schedule.
- `Preference`: likes and dislikes with evidence and confidence.
- `EmotionState`: recent emotional observations.
- `ImportantEvent`: user events such as interviews, exams, moving, deadlines.
- `RelationshipPreference`: preferred agent style.
- `RelationshipState`: familiarity, trust, intimacy, and turn count.
- `MemoryItem`: append-only history of memory changes.
- `MemoryConflict`: explicit conflict record when a new value overwrites an old one.
- `SessionState`: short-term in-process recent turns.

## Tool Design

The runtime depends on interfaces:

- `ExtractionTool`: extracts facts from natural language.
- `MemoryTool`: loads, updates, and saves memory.

Current implementations:

- `RuleBasedExtractor`: deterministic, zero-dependency extractor.
- `PersistentMemoryTool`: adapter around the memory store.
- `JSONStore`: local JSON persistence.

This keeps the runtime independent from a specific model or database. A future LLM extractor or SQL store can be introduced without rewriting `AgentRuntime`.

## Conflict Policy

For scalar basic fields, the latest user statement becomes current truth. The old value is not deleted; it is saved in `MemoryHistory` and `Conflicts`.

Example:

- Old: `basic_info.city = 上海`
- New: `basic_info.city = 深圳`
- Current profile uses 深圳.
- Conflict history records 上海 -> 深圳 and the evidence message.

This simple policy is suitable for a minimal runtime because it is predictable and explainable.
