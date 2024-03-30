# 10: Voracious Code Storage

[View leaderboard](https://protohackers.com/leaderboard/10)
Thu, 19 Jan 2023 12:00:00

**Voracious Code Storage** is a version control system accessed over the Internet. Clients can insert and retrieve text files. Each revision of a file is identified by its filename and a revision number.

Your job is to implement a VCS server.

The only known implementation has been lost, apart from a **trial copy** that remains available at **vcs.protohackers.com** on **port 30307**, which you may like to experiment with as part of your reverse engineering effort. The trial copy has the following limitations:

- it is very slow
- there is a low limit on maximum file size
- each session gets its own private temporary storage, instead of a shared global persistent storage

Your server must not have these limitations.

Sadly nobody knows how the protocol works, no client implementation exists, and no documentation is available.

Good luck!

## Reverse Engineered Spec

- Each message is delimited by a newline `\n`.
- Messages should be interpreted as ASCII
- Each client message begins with a _method_ defined below:
- After serving each request message, the server should respond with a `READY` line to inform the client it's ready for more requests.

### Revision Number

A _revision number_ begins with a `r` followed by an `int`. Revision numbers start at `r1`. Each `PUT` to the same _filepath_ increments the revision number.

### Methods

#### `HELP`:

Request usage information. No args. The server should respond with a `OK` line followed by a usage message containing the available request methods, delimited by `|` character.

```
<-- HELP\n
--> OK usage: HELP|GET|PUT|LIST\n
```

#### `PUT`:

`PUT` messages consist of the following parts, separated by a single space ` ` character. The first line of a `PUT` message is the header, which contains the `PUT`, _filepath_, and _content-length_. The subsequent line contains the file contents and should be the same length as _content-length_ in the header.
Each `PUT` request should increment the revision number for the given _filepath_, _unless the file contents are identical to the previous revision_.

| Description                                   | Type     | Ex. value   |
| --------------------------------------------- | -------- | ----------- |
| method                                        | `string` | `PUT`       |
| filepath                                      | `string` | `/test.txt` |
| file content length (including trailing `\n`) | `int`    | `3`         |
| file content                                  | `string` | `hi\n`      |

Ex:

```
--> READY\n
<-- PUT /test.txt 14\n
<-- Hello, world!\n
--> OK r1\n
--> READY
```

The server should respond with an `OK` followed by the _revision number_.

#### `GET`:

`GET` messages contain the method `GET`, the _filepath_ and an optional _revision number_. If the revision number is omitted, the _latest revision_ is returned, otherwise the revision requested by the client is returned.
The server should respond with

- a line containing `OK` and the the revision's content-length, including the trailing newline
- the file contents on the next line
- finally a `READY` line

Examples:

Revision number provided

```
<-- GET /text.txt r1\n
--> OK 14\n
--> hello, world!\n
--> READY\n
```

Revision number omitted. Latest revision returned:

```
<-- GET /text.txt\n
--> OK 8\n
--> bonjour\n
--> READY\n
```

#### `LIST`:

`LIST` messages contain the method `LIST` followed by a directory. The server should respond with a `OK` line followed by an _int_ corresponding to the number of entries in the given directory.
The next line(s) should list the entries, one entry per line. If the entry is a file, the listing contains the **file name** and the **latest revision number**. If the listing is a directory, the line contains the directory's name and the string `"DIR"`. The list should _alphabetically sorted_ by the entry's name.
The list should be terminated by a `READY` line.

Example:
For a directory containing two files and one directory:

```
/
├── dir1
├── test.txt
└── test2.txt
```

```
LIST /
OK 3
dir1/ DIR
test.txt r1
test2.txt r1
```

### Example Session

Client messages denoted with `<--`. Server responses denoted with `-->`.

```

--> READY\n
<-- PUT /test.txt 14\n
<-- hello, world!\n
--> OK r1\n
--> READY\n
<-- PUT /test.txt 17\n
<-- ¡hola, la gente!\n
--> OK r2\n
--> READY\n
<-- GET /text.txt r1\n
--> OK 14\n
--> hello, world!\n
--> READY\n

```
