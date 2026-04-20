# Memory Behavior Fix

This note documents the latest runtime behavior fix.

## Fixed Scenario

The CLI previously failed to extract this input:

```text
我是Tony 我是一名CEO
```

The runtime now extracts:

```json
{
  "basic_info": {
    "name": "Tony",
    "occupation": "CEO"
  }
}
```

A later memory question:

```text
你能记得我的职业吗？
```

now returns:

```text
记得，你的职业是CEO。如果之后有变化，你直接告诉我，我会更新记忆。
```

## Additional Covered Behavior

- English names after `我是` / `我叫`
- Executive occupations such as `CEO`, `CTO`, `CFO`, `COO`
- Direct memory queries for name, occupation, city, preferences, and general profile
- Conflict updates such as `上海 -> 深圳`

## Verification

Run:

```powershell
go test ./...
```

or use the CLI:

```powershell
.\relationship-agent-cli.exe --user fixed-demo --memory data\memory
```

Example turns:

```text
我是Tony 我是一名CEO
你能记得我的职业吗？
我现在在上海，我喜欢咖啡，希望你温柔一点
其实我已经搬到深圳了
```
