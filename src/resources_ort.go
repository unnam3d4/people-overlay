//go:build !ncnn
package main
func resourceFiles()[]string{return []string{"model.onnx","onnxruntime.dll"}}
