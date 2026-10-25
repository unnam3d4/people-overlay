package main

import "sync/atomic"

// An arbitrary rectangular grid, not a fixed '50 cameras' assumption.
// A physical display has one common layout; different displays may still
// have different layouts through the existing automatic profiles.
const maxCustomGridAxis = 12
var customGridColumns atomic.Int32
var customGridRows atomic.Int32
func validGridAxis(value int) int {
    if value<1||value>maxCustomGridAxis{return 4}
    return value
}
func manualGridDimensions()(columns,rows int){
    return validGridAxis(int(customGridColumns.Load())),
           validGridAxis(int(customGridRows.Load()))
}
