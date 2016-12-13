# apd

apd is an arbitrary-precision decimal package for Go.

## **WARNING**

This library should not be used in production. It is under development, may change, and is not tested in a real environment.

## Documentation

https://godoc.org/github.com/mjibson/apd

## Goals

- panic-free operation; use errors when necessary
- defined performance (speed of operations can be defined by, i.e., size of input and precision)
- accurate precision (an operation performed with requested precision will use enough precision during internal operations to achieve desired result)

## Testing

Testing is done primarily with the suite from [General Decimal Arithmetic](http://speleotrove.com/decimal/), which contains thousands of tests for various operations.
