# ReHook [![GoDoc][doc-img]][doc] [![Build Status][ci-img]][ci] [![Coverage Status][cov-img]][cov]

A utility that allows you to test third-party service (stripe, github, paypal, etc...) webhooks locally.

## Installation

`go get -u github.com/sintanial/go-rehook`

## How it work

Everything is very simple. 
You must start the server on a remote machine that will listen on the specified port. After that, you need to run the client locally and specify the server address and the rules by which web hooks will be relayed. 

The client will connect to the server via the web socket. 
When a request from a third-party service arrives at the server, the server will send this request via web sockets to the client.
The client according to the specified rules will send a request to the specified address, wait for a response and send it to the server. 
The server will send a response to the remote service.

## Quick Start

### Step 1: Run server on the remote machine (for example 10.1.2.3)
```bash
rehook_server -addr 10.1.2.3:8181
```

### Step 2: Write relayed rules. 
As the key, you must specify the path to which the third-party service will send the request. 
In the value, the address where to relay the request.

For example:
```yaml
rules:
    /webhook/test/foo: http://localhost:8080/webhook/test/foo
    /another/path/bar: http://localhost:8080/webhook/test/bar
```

### Step 3: Run client on the local machine
```bash
rehook_client -addr 10.1.2.3:8181 -rules /path/to/rules.yaml
```

### Step 4: Check that everything is working fine.
```bash
# all query parameters, headers, body are keeped

curl -v -L "http://10.1.2.3:8181/webhook/test/foo?hello=world"
```

### Step 5: Register webhook retranslator address *http://10.1.2.3:8181/webhook/test/foo?hello=world* to third-party service (for example stripe)
