/*
 *
 * Copyright 2014, Google Inc.
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 *     * Redistributions of source code must retain the above copyright
 * notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above
 * copyright notice, this list of conditions and the following disclaimer
 * in the documentation and/or other materials provided with the
 * distribution.
 *     * Neither the name of Google Inc. nor the names of its
 * contributors may be used to endorse or promote products derived from
 * this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 *
 */

package grpc

import (
	"bytes"
	"io"
	"math"
	"time"

    "fmt"

	"golang.org/x/net/context"
	"golang.org/x/net/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/transport"
)

// recvResponse receives and parses an RPC response.
// On error, it returns the error and indicates whether the call should be retried.
//
// TODO(zhaoq): Check whether the received message sequence is valid.
func recvResponse(dopts dialOptions, t transport.ClientTransport, c *callInfo, stream *transport.Stream, reply interface{}) error {
    //fmt.Println("vendor/google/grpc/call.go  recvResponse()")
    if stream.StatusCode() != 0 {
         fmt.Println("vendor/google/grpc/call.go  recvResponse() begin StatusCode : ", stream.StatusCode())
         fmt.Println("vendor/google/grpc/call.go  recvResponse() begin StatusDesc : ", stream.StatusDesc())
    }

	// Try to acquire header metadata from the server if there is any.
	var err error
	defer func() {
		if err != nil {
			if _, ok := err.(transport.ConnectionError); !ok {
				t.CloseStream(stream, err)
			}
		}
	}()

	c.headerMD, err = stream.Header()
    if stream.StatusCode() != 0 {
         fmt.Println("vendor/google/grpc/call.go  recvResponse() afterHeader StatusCode : ", stream.StatusCode())
         fmt.Println("vendor/google/grpc/call.go  recvResponse() afterHeader StatusDesc : ", stream.StatusDesc())
         fmt.Println("vendor/google/grpc/call.go  recvResponse() afterHeader c.headerMD : ", c.headerMD)
         fmt.Println("vendor/google/grpc/call.go  recvResponse() afterHeader err : ", err)
    }

	if err != nil {
		return err
	}

	p := &parser{r: stream}
    if stream.StatusCode() != 0 {
         fmt.Println("vendor/google/grpc/call.go  recvResponse() afterParser StatusCode : ", stream.StatusCode())
         fmt.Println("vendor/google/grpc/call.go  recvResponse() afterParser StatusDesc : ", stream.StatusDesc())
    }

	for {

        if stream.StatusCode() != 0 {
            fmt.Println("vendor/google/grpc/call.go  recvResponse() beforeRecv StatusCode : ", stream.StatusCode())
            fmt.Println("vendor/google/grpc/call.go  recvResponse() beforeRecv StatusDesc : ", stream.StatusDesc())
        }

        //fmt.Println("vendor/google/grpc/call.go  recvResponse() before recv")
		if err = recv(p, dopts.codec, stream, dopts.dc, reply, math.MaxInt32); err != nil {
			if err == io.EOF {
				break
			}
            fmt.Println("vendor/google/grpc/call.go  recvResponse() afterRecv is err : ", err)
			return err
		}

        if stream.StatusCode() != 0 {
           fmt.Println("vendor/google/grpc/call.go  recvResponse() afterRecv StatusCode : ", stream.StatusCode())
           fmt.Println("vendor/google/grpc/call.go  recvResponse() afterRecv StatusDesc : ", stream.StatusDesc())
        }
	}

	c.trailerMD = stream.Trailer()
	return nil
}

// sendRequest writes out various information of an RPC such as Context and Message.
func sendRequest(ctx context.Context, codec Codec, compressor Compressor, callHdr *transport.CallHdr, t transport.ClientTransport, args interface{}, opts *transport.Options) (_ *transport.Stream, err error) {
	//fmt.Println("vendor/google.golang.org/grpc/call.go  sendRequest()")
	//fmt.Println("vendor/google.golang.org/grpc/call.go  sendRequest() before NewStream")
    stream, err := t.NewStream(ctx, callHdr)
    if stream.StatusCode() != 0 {
    fmt.Println("vendor/google/grpc/call.go  sendRequest()")
    fmt.Println("vendor/google/grpc/call.go  sendRequest() new  stream.StatusCode : ", stream.StatusCode())
    fmt.Println("vendor/google/grpc/call.go  sendRequest() new  stream.StatusDesc : ", stream.StatusDesc())
    }

	if err != nil {
        fmt.Println("vendor/google/grpc/call.go  sendRequest() err : ", err)
		return nil, err
	}

	defer func() {
		if err != nil {
			// If err is connection error, t will be closed, no need to close stream here.
			if _, ok := err.(transport.ConnectionError); !ok {
                //fmt.Println("vendor/google/grpc/call.go  sendRequest() err : ", transport.ConnectionError)
				t.CloseStream(stream, err)
			}
		}
      if stream.StatusCode() != 0 {
        fmt.Println("vendor/google/grpc/call.go  sendRequest() defer  stream.StatusCode : ", stream.StatusCode())
        fmt.Println("vendor/google/grpc/call.go  sendRequest() defer  stream.StatusDesc : ", stream.StatusDesc())
      }
	}()

	var cbuf *bytes.Buffer
	if compressor != nil {
		cbuf = new(bytes.Buffer)
	}
	outBuf, err := encode(codec, args, compressor, cbuf)
	if err != nil {
		return nil, Errorf(codes.Internal, "grpc: %v", err)
	}

    if stream.StatusCode() != 0 {
    fmt.Println("vendor/google/grpc/call.go  sendRequest() beforeWrite  stream.StatusCode : ", stream.StatusCode())
    fmt.Println("vendor/google/grpc/call.go  sendRequest() beforeWrite  stream.StatusDesc : ", stream.StatusDesc())
    }


    if stream.Method() == "/types.API/AddProcess" {
    fmt.Println("vendor/google/grpc/call.go  sendRequest() beforeWrite  stream : ", stream)
    fmt.Println("vendor/google/grpc/call.go  sendRequest() beforeWrite  outBuf : ", outBuf)
    fmt.Println("vendor/google/grpc/call.go  sendRequest() beforeWrite  opts : ", opts)
    fmt.Println("vendor/google/grpc/call.go  sendRequest() beforeWrite  ctx : ", ctx)
    fmt.Println("vendor/google/grpc/call.go  sendRequest() beforeWrite  callHdr : ", callHdr)
    fmt.Println("vendor/google/grpc/call.go  sendRequest() beforeWrite  t : ", t)
    }
    //fmt.Println("vendor/google/grpc/call.go  sendRequest() before Write")
	err = t.Write(stream, outBuf, opts)
    if stream.StatusCode() != 0 {
    fmt.Println("vendor/google/grpc/call.go  sendRequest() afterWrite  stream.StatusCode : ", stream.StatusCode())
    fmt.Println("vendor/google/grpc/call.go  sendRequest() afterWrite  stream.StatusDesc : ", stream.StatusDesc())
    }

	// t.NewStream(...) could lead to an early rejection of the RPC (e.g., the service/method
	// does not exist.) so that t.Write could get io.EOF from wait(...). Leave the following
	// recvResponse to get the final status.
	if err != nil && err != io.EOF {
		return nil, err
	}
	// Sent successfully. 

    //fmt.Println("vendor/google/grpc/call.go  sendRequest() end stream.method : ", stream.Method())
    return stream, nil
}

// Invoke sends the RPC request on the wire and returns after response is received.
// Invoke is called by generated code. Also users can call Invoke directly when it
// is really needed in their use cases.
func Invoke(ctx context.Context, method string, args, reply interface{}, cc *ClientConn, opts ...CallOption) error {
    //fmt.Println("vendor/google/grpc/call.go  Invoke()")
	if cc.dopts.unaryInt != nil {
        fmt.Println("vendor/google/grpc/call.go  Invoke() is err!!!")
		return cc.dopts.unaryInt(ctx, method, args, reply, cc, invoke, opts...)
	}
	return invoke(ctx, method, args, reply, cc, opts...)
}

func invoke(ctx context.Context, method string, args, reply interface{}, cc *ClientConn, opts ...CallOption) (err error) {
    //fmt.Println("vendor/google/grpc/call.go  invoke()")
    if method == "/types.API/AddProcess" {
       fmt.Println("vendor/google/grpc/call.go  invoke() method /types.API/AddProcess")
       fmt.Println("vendor/google/grpc/call.go  invoke() defaultCallInfo : ", defaultCallInfo)
    }

	c := defaultCallInfo
	for _, o := range opts {
		if err := o.before(&c); err != nil {
    fmt.Println("vendor/google/grpc/call.go  invoke()  o.before()")
			return toRPCErr(err)
		}
	}
	defer func() {
		for _, o := range opts {
			o.after(&c)
		}
	}()
	if EnableTracing {
    if method == "/types.API/AddProcess" {
       fmt.Println("vendor/google/grpc/call.go  invoke() EnableTracing : ", EnableTracing)
    }
		c.traceInfo.tr = trace.New("grpc.Sent."+methodFamily(method), method)
		defer c.traceInfo.tr.Finish()
		c.traceInfo.firstLine.client = true
		if deadline, ok := ctx.Deadline(); ok {
			c.traceInfo.firstLine.deadline = deadline.Sub(time.Now())
		}
		c.traceInfo.tr.LazyLog(&c.traceInfo.firstLine, false)
		// TODO(dsymonds): Arrange for c.traceInfo.firstLine.remoteAddr to be set.
		defer func() {
			if err != nil {
				c.traceInfo.tr.LazyLog(&fmtStringer{"%v", []interface{}{err}}, true)
				c.traceInfo.tr.SetError()
			}
		}()
	}
	topts := &transport.Options{
		Last:  true,
		Delay: false,
	}
	for {
		var (
			err    error
			t      transport.ClientTransport
			stream *transport.Stream
			// Record the put handler from Balancer.Get(...). It is called once the
			// RPC has completed or failed.
			put func()
		)
		// TODO(zhaoq): Need a formal spec of fail-fast.
		callHdr := &transport.CallHdr{
			Host:   cc.authority,
			Method: method,
		}
		if cc.dopts.cp != nil {
			callHdr.SendCompress = cc.dopts.cp.Type()
		}
		gopts := BalancerGetOptions{
			BlockingWait: !c.failFast,
		}
		t, put, err = cc.getTransport(ctx, gopts)
		if err != nil {
            fmt.Println("vendor/google/grpc/call.go  invoke()  cc.getTransport()")
			// TODO(zhaoq): Probably revisit the error handling.
			if _, ok := err.(*rpcError); ok {
            fmt.Println("vendor/google/grpc/call.go  invoke()  err.rpcError")
				return err
			}
			if err == errConnClosing || err == errConnUnavailable {
				if c.failFast {
                    fmt.Println("vendor/google/grpc/call.go  invoke()  codes.unavailable")
					return Errorf(codes.Unavailable, "%v", err)
				}
				continue
			}
			// All the other errors are treated as Internal errors.
			return Errorf(codes.Internal, "%v", err)
		}
		if c.traceInfo.tr != nil {
			c.traceInfo.tr.LazyLog(&payload{sent: true, msg: args}, true)
		}

        
        if method == "/types.API/AddProcess" {
           fmt.Println("vendor/google/grpc/call.go  invoke()  before sendrequest()")
//           fmt.Println("vendor/google/grpc/call.go  invoke()  stream.StatusCode : ", stream.StatusCode())
//           fmt.Println("vendor/google/grpc/call.go  invoke()  stream.StatusDesc : ", stream.StatusDesc())
           fmt.Println("vendor/google/grpc/call.go  invoke()  ctx : ", ctx)
           fmt.Println("vendor/google/grpc/call.go  invoke()  cc.dopts.codec : ", cc.dopts.codec)
           fmt.Println("vendor/google/grpc/call.go  invoke()  cc.dopts.cp : ", cc.dopts.cp)
           fmt.Println("vendor/google/grpc/call.go  invoke()  callHdr : ", callHdr)
           fmt.Println("vendor/google/grpc/call.go  invoke()  t : ", t)
           fmt.Println("vendor/google/grpc/call.go  invoke()  args : ", args)
           fmt.Println("vendor/google/grpc/call.go  invoke()  topts : ", topts)
        }

        //fmt.Println("vendor/google/grpc/call.go  invoke() before sendRequest")
		stream, err = sendRequest(ctx, cc.dopts.codec, cc.dopts.cp, callHdr, t, args, topts)
        if method == "/types.API/AddProcess" {
           fmt.Println("vendor/google/grpc/call.go  invoke()  after sendrequest()")
           fmt.Println("vendor/google/grpc/call.go  invoke()  stream.StatusCode : ", stream.StatusCode())
           fmt.Println("vendor/google/grpc/call.go  invoke()  stream.StatusDesc : ", stream.StatusDesc())
        }

		if err != nil {
            fmt.Println("vendor/google/grpc/call.go  invoke()  sendrequest() err!!!")
			if put != nil {
				put()
				put = nil
			}
			// Retry a non-failfast RPC when
			// i) there is a connection error; or
			// ii) the server started to drain before this RPC was initiated.
			if _, ok := err.(transport.ConnectionError); ok || err == transport.ErrStreamDrain {
				if c.failFast {
                    fmt.Println("vendor/google/grpc/call.go  invoke()  connectionError")
					return toRPCErr(err)
				}
				continue
			}
			return toRPCErr(err)
		}

        if method == "/types.API/AddProcess" {
           fmt.Println("vendor/google/grpc/call.go  invoke()  before recvResponse()")
           fmt.Println("vendor/google/grpc/call.go  invoke()  stream.StatusCode : ", stream.StatusCode())
           fmt.Println("vendor/google/grpc/call.go  invoke()  stream.StatusDesc : ", stream.StatusDesc())
        }
        //fmt.Println("vendor/google/grpc/call.go  invoke()  before recvResponse()")
		err = recvResponse(cc.dopts, t, &c, stream, reply)
        if method == "/types.API/AddProcess" {
           fmt.Println("vendor/google/grpc/call.go  invoke()  after recvResponse()")
           fmt.Println("vendor/google/grpc/call.go  invoke()  stream.StatusCode : ", stream.StatusCode())
           fmt.Println("vendor/google/grpc/call.go  invoke()  stream.StatusDesc : ", stream.StatusDesc())
        }
		if err != nil {
            fmt.Println("vendor/google/grpc/call.go  invoke()  recvResponse err!!!")
			if put != nil {
				put()
				put = nil
			}
			if _, ok := err.(transport.ConnectionError); ok || err == transport.ErrStreamDrain {
				if c.failFast {
					return toRPCErr(err)
				}
				continue
			}
			return toRPCErr(err)
		}
		if c.traceInfo.tr != nil {
			c.traceInfo.tr.LazyLog(&payload{sent: false, msg: reply}, true)
		}
		t.CloseStream(stream, nil)
		if put != nil {
			put()
			put = nil
		}
        if method == "/types.API/AddProcess" {
           fmt.Println("vendor/google/grpc/call.go  invoke()  stream.StatusCode : ", stream.StatusCode())
           fmt.Println("vendor/google/grpc/call.go  invoke()  stream.StatusDesc : ", stream.StatusDesc())
        }
		return Errorf(stream.StatusCode(), "%s", stream.StatusDesc())
	}
}
