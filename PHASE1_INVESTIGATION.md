# Tailwatch Phase 1: Event Collection Investigation

## Executive Summary

After comprehensive investigation of Tailscale SSH event collection capabilities, I must report a **critical limitation**: **No reliable local event source exists for Tailscale SSH sessions** in the current Tailscale ecosystem (as of version 1.48.0+).

This is a fundamental constraint that must be documented and addressed in the architecture design.

## Investigation Findings

### 1. Tailscale SSH Architecture

**What Tailscale SSH Does:**
- Intercepts TCP port 22 for traffic arriving **through the tailnet** only
- Uses the Tailscale IP address range (`100.64.0.0/10`) as the destination
- Routes SSH traffic through an internal Tailscale SSH server
- Requires Tailscale SSH access control policies for authorization

**What Tailscale SSH Does NOT Provide:**
- Local SSH session logging
- Event emission for monitoring tools
- System log integration
- Event APIs for external consumers

### 2. Current Event Collection Options

**Option 1: System Logs (`journalctl -u tailscaled`)**
- **Status**: Limited to general Tailscale daemon logs
- **Content**: Connection state changes, peer updates, general errors
- **SSH Events**: **NOT INCLUDED** - No SSH session start/end events
- **Reliability**: Sufficient for operational monitoring but not security events

**Option 2: Tailscale SSH Recording**
- **Status**: Available but separate feature
- **Purpose**: Recording SSH sessions for audit
- **Integration**: Requires separate setup and storage
- **Event Format**: Different from monitoring requirements

**Option 3: Tailscale API**
- **Status**: Available via control plane
- **Access**: Requires authentication to Tailscale control
- **Event Lag**: Not real-time local monitoring
- **Scope**: Network-level events, not SSH-specific

### 3. Critical Technical Constraint

**Issue**: The fundamental design of Tailscale SSH does not emit observable events at the local system level for monitoring daemons to consume.

**Root Cause**: Tailscale SSH operates as a transparent proxy - it accepts SSH connections and forwards them internally without logging session metadata.

**Impact**: Cannot detect, classify, or respond to SSH access events at the network boundary using local monitoring tools.

## Required Architecture Adjustments

### 1. Event Source Abstraction
We must implement a flexible event source interface that can:
- Accept real events when a supported source becomes available
- Use test/fake events for development and testing
- Provide diagnostic information about event availability

### 2. Fallback Event Collection Strategy
- Monitor Tailscale daemon logs for indirect SSH activity indicators
- Correlate with system authentication logs when possible
- Implement manual test mode for development

### 3. Diagnostic and Monitoring Capabilities
- Provide clear visibility into event collection status
- Offer guidance for setting up alternative monitoring approaches
- Include logging about event source availability

## Immediate Action Items

### Phase 1.1: Event Source Framework
```go
// Event source interface for flexibility
package source

type EventSource interface {
    Start(ctx context.Context) error
    Stop() error
    GetEvents(ctx context.Context) (<-chan Event, error)
}

// Test event source for development
package source

type TestEventSource struct {}
```

### Phase 1.2: Diagnostic Command
Implement a command that:
1. Checks for Tailscale event logging support
2. Reports available event sources
3. Suggests alternatives if none available
4. Provides test event options for development

### Phase 1.3: Architecture Documentation
Document the event collection limitations clearly in:
- README.md
- Architecture documentation
- Deployment guides
- Troubleshooting documentation

## Future Integration Possibilities

### Option A: Enhanced Tailscale Event Logging
- Monitor for Tailscale feature releases
- Integrate when SSH session logging becomes available
- Maintain backward compatibility with current architecture

### Option B: Third-Party Solutions
- Investigate SIEM integration with Tailscale logs
- Explore network monitoring solutions
- Consider endpoint detection solutions

### Option C: Direct API Integration
- Develop API-based event collection
- Implement secure, authenticated event polling
- Consider rate limiting and access control

## Testing and Development Strategy

### Development Mode
- Use TestEventSource for all development
- Mock identity resolution for testing
- Simulate various trust scenarios
- Validate classification logic independently

### Testing Mode
- Unit test classification algorithms
- Test identity resolution failures
- Validate notification formatting
- Test error handling scenarios

### Production Mode
- Monitor Tailscale event logging support
- Implement real event source when available
- Provide clear migration path

## Recommended Implementation Sequence

### Week 1: Core Framework
1. Implement EventSource interface
2. Create TestEventSource
3. Add diagnostic command
4. Update configuration system

### Week 2: Application Integration
1. Integrate event source into main app
2. Implement test data pipeline
3. Add logging and diagnostics
4. Create testing fixtures

### Week 3: Classification and Notification
1. Implement device classification
2. Build notification system
3. Test event processing pipeline
4. Create integration tests

## Known Limitations to Document

### In Current Release
- No local SSH event logging available
- Cannot detect SSH session boundaries
- Limited to indirect indicators
- Requires manual testing with simulated events

### Mitigation Strategies
- Use simulated events for development
- Implement comprehensive unit testing
- Document event collection limitations clearly
- Provide clear setup and usage instructions

## Conclusion

The Tailscale SSH event collection investigation has revealed a **fundamental architectural limitation** that must be explicitly addressed in the Tailwatch design. The application cannot rely on native event logging and must:

1. **Be built around this limitation**
2. **Provide clear documentation about constraints**
3. **Offer development alternatives**
4. **Plan for future integration**
5. **Maintain testability and observability**

Despite this challenge, the core monitoring functionality (device classification, identity resolution, and notification) can be fully implemented and tested, with the event source integration being a modular component that can be enhanced when Tailscale provides proper event logging support.

**Priority**: Continue with core implementation while documenting event collection constraints clearly for end users.
