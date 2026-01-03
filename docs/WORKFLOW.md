# CTO Workflow for Phase 1 Execution

## Roles
- You: CTO/reviewer.
- Claude: executor who reads tasks, proposes plans, implements code.

## Task Lifecycle (Per Task)
1. CTO creates a task folder under `workflow/` with name `taskN/`.
2. CTO writes requirements to `workflow/taskN/taskN.md`.
3. Claude reads `taskN.md` and writes a plan to `workflow/taskN/plan.md`.
4. CTO reviews `plan.md` and edits if needed.
5. Claude reads `approve.md`:
   - If not `true`, Claude reads the latest plan and revises it.
   - This repeats until `approve.md` contains `true`.
6. Claude implements code for the task.
7. CTO reviews the code and writes feedback in `workflow/taskN/review.md`.
8. Claude checks `review.approve`:
   - If not `true`, Claude fixes issues and re-submits.
   - This repeats until `review.approve` contains `true`.
9. Only after task completion, proceed to the next task.

## Definition of Done (per task)
- `approve.md` is `true` for the plan.
- `review.approve` is `true` after review.
- Task requirements satisfied and verified.

## Permissions
- The user grants permission for all bash commands needed to execute the workflow.

## Notes
- All collaboration artifacts live in `workflow/`.
- Do not advance to the next task until the current one is complete.
