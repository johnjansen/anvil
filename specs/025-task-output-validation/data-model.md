# Data Model: Task Output Validation with Assertions

## Entities

### AssertionConfig

Configuration for task output assertions.

**Fields**:
- `Stdout` (StdoutAssertions): Assertions to apply to stdout output
- `Stderr` (StderrAssertions): Assertions to apply to stderr output
- `Files` ([]FileAssertion): Array of file assertions
- `Soft` (bool): Whether assertion failures should be treated as warnings rather than errors

### StdoutAssertions

Assertions to apply to stdout output.

**Fields**:
- `Contains` (string): String that must be present in stdout
- `Matches` (string): Regular expression pattern that stdout must match
- `JSONValid` (bool): Whether stdout must be valid JSON
- `Empty` (bool): Whether stdout must be empty

### StderrAssertions

Assertions to apply to stderr output.

**Fields**:
- `Contains` (string): String that must be present in stderr
- `Matches` (string): Regular expression pattern that stderr must match
- `Empty` (bool): Whether stderr must be empty

### FileAssertion

Assertion to apply to a file.

**Fields**:
- `Path` (string): Path to the file to check
- `Exists` (bool): Whether the file must exist
- `Contains` (string): String that must be present in the file
- `SizeMin` (int): Minimum file size in bytes
- `SizeMax` (int): Maximum file size in bytes

## Relationships

- `AssertionConfig` belongs to `Todo` (task configuration)
- `AssertionConfig` contains `StdoutAssertions`, `StderrAssertions`, and `[]FileAssertion`
- `StdoutAssertions` and `StderrAssertions` are similar but applied to different output streams
- `FileAssertion` represents individual file checks that are part of the overall assertion configuration

## Validation Rules

From requirements:
- At least one assertion field must be specified when `Assert` is configured
- Regex patterns in `Matches` must be valid Go regular expressions
- File paths in `FileAssertion` must be valid file paths
- `SizeMin` must be >= 0
- `SizeMax` must be >= 0
- If both `SizeMin` and `SizeMax` are specified, `SizeMin` must be <= `SizeMax`
- `Soft` defaults to false if not specified

## State Transitions

Assertion evaluation happens after task execution:
1. Task executes normally
2. Output (stdout, stderr) is captured
3. Assertion evaluation begins
4. All configured assertions are evaluated
5. If any hard assertion fails, task is marked failed
6. If only soft assertions fail, task succeeds but warnings are logged
7. Task completion status is recorded in run record