# Data Model for Task Timeout Escalation

## Entities

### Todo (extends existing entity)

The existing Todo struct will be extended with new fields for timeout escalation:

#### Fields

- **TimeoutWarning** (time.Duration): Duration before timeout when warning should be triggered
- **OnTimeoutWarning** (string): Shell command to execute when timeout warning is triggered
- **OnTimeout** (string): Shell command to execute when task times out (in addition to existing behavior)
- **AdaptiveTimeout** (*AdaptiveTimeoutConfig): Configuration for adaptive timeout behavior

#### Relationships

The Todo entity relates to:
- RunningTask in the daemon (for monitoring timeout status)
- Project configuration (for default values)

### AdaptiveTimeoutConfig

New configuration struct for adaptive timeout behavior:

#### Fields

- **Enabled** (bool): Whether adaptive timeout is enabled
- **ExtendIf** (string): Condition that triggers timeout extension ("checkpoint_exists", etc.)
- **MaxExtensions** (int): Maximum number of timeout extensions allowed
- **ExtensionDuration** (time.Duration): Duration to extend timeout by (defaults to original timeout)

### RunningTask (extends existing entity)

The existing RunningTask struct in the daemon will be enhanced to track timeout escalation:

#### Fields

- **WarningTriggered** (bool): Whether the timeout warning has been triggered
- **TimeoutExtensions** (int): Number of timeout extensions used
- **OriginalTimeout** (time.Duration): Original timeout value for reference