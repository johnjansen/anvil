# Research: Message Queue Subscription for Task Triggers

**Feature**: Message Queue Subscription for Task Triggers
**Date**: 2026-03-01

## Research Summary

This feature follows the same pattern as the filesystem subscription (spec 001), with technology choices matching existing anvil patterns.

## Technology Decisions

### AMQP Library

**Decision**: Use `rabbitmq/amqp091-go` (official RabbitMQ Go client)

**Rationale**:
- Standard AMQP 0.9.1 implementation (required by spec FR-006)
- Well-maintained and widely used
- Compatible with RabbitMQ and other AMQP 0.9.1 brokers (ActiveMQ, Qpid)
- Same approach as other Go AMQP projects

**Alternatives considered**:
- `streadway/amqp` (older, less maintained)
- `googlecloudplatform/cloudamqp` (cloud-specific)

### Connection URL Format

**Decision**: Standard AMQP URL format: `amqp://user:pass@host:port/vhost`

**Rationale**:
- Industry standard format
- Supported by all AMQP brokers
- Matches URL format used by RabbitMQ documentation

### Message Acknowledgment

**Decision**: Support both auto-ack and manual ack modes

**Rationale**:
- Auto-ack: Simple, fire-and-forget (FR-009)
- Manual ack: Allows task to complete before message is acknowledged
- Configurable via subscription settings

### Queue Declaration

**Decision**: Support optional queue declaration (create if not exists)

**Rationale**:
- Default behavior: declare queue if not exists (FR-008)
- Allows pre-existing queue usage
- Matches RabbitMQ common patterns

## Implementation Approach

The AMQP subscription implementation will mirror the filesystem subscription pattern:

1. **Subscription struct**: Implements the subscription interface with AMQP-specific logic
2. **Connection management**: Handle connection, reconnection, and error states
3. **Consumer**: Set up message consumer with delivery handling
4. **Event conversion**: Convert AMQP deliveries to task trigger events

## References

- [RabbitMQ AMQP 0.9.1 Reference](https://www.rabbitmq.com/amqp-0-9-1-reference.html)
- [amqp091-go library](https://github.com/rabbitmq/amqp091-go)
- [Existing filesystem subscription implementation](./001-filesystem-subscription/)
