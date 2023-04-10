# 7: Line Reversal

[View leaderboard](https://protohackers.com/leaderboard/7)
Thu, 17 Nov 2022 12:00:00

We're going to be writing a simple network server to reverse the characters within lines of ASCII text. For example, we'll turn "hello" into "olleh".

There's just one snag: we've never heard of TCP! Instead, we've designed our own connection-oriented byte stream protocol that runs on top of UDP, called "Line Reversal Control Protocol", or **LRCP** for short.

The goal of LRCP is to turn unreliable and out-of-order UDP packets into a pair of **reliable and in-order byte streams**. To achieve this, it maintains a per-session **payload length counter on each side**, labels all payload transmissions with their **position in the overall stream**, and retransmits any data that has been dropped. A sender detects that a packet has been dropped either by not receiving an acknowledgment within an expected time window, or by receiving a duplicate of a prior acknowledgement.

Client sessions are identified by a numeric session token which is supplied by the client. You can assume that **session tokens uniquely identify client**s, and that the peer for any given session is at a fixed IP address and port number.

## Messages

Messages are sent in **UDP packets**. Each UDP packet contains a single LRCP message. Each message consists of a series of **values separated by forward slash** characters (`"/"`), and starts and ends with a forward slash character, like so:

```
/data/1234567/0/hello/
```

The first field is a string specifying the message type (here, `"data"`). The remaining fields depend on the message type. Numeric fields are represented as ASCII text.

### Validation

When the server receives an illegal packet it must **silently ignore the packet** instead of interpreting it as LRCP.

1. Packet contents must begin with a forward slash, end with a forward slash, have a valid message type, and have the correct number of fields for the message type.
2. Numeric field values must be smaller than 2147483648. This means sessions are limited to 2 billion bytes of data transferred in each direction.
3. LRCP messages must be smaller than 1000 bytes. You might have to break up data into multiple data messages in order to fit it below this limit.

### Parameters

- **retransmission timeout:** the time to wait before retransmitting a message. Suggested default value: **3 seconds**.

- **session expiry timeout:** the time to wait before accepting that a peer has disappeared, in the event that no responses are being received. Suggested default value: **60 seconds**.
