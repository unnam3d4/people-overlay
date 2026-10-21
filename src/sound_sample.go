package main

import (
 "bytes"
 "compress/zlib"
 "encoding/base64"
 "encoding/binary"
 "fmt"
 "io"
)

// A single alarm pulse, sampled directly from the user-provided
// aviation-alarm.mp3 (8 kHz mono 8-bit PCM). Its eight original pulses are
// near-identical and occur about 310 ms apart. Repeating this exact sample
// preserves the requested sound while adding only ~1 KB to the EXE.
const originalAlarmPulseZlib64 = "eNrllF1u20YUhTfR2EFfu4q89tFygC6gSUhJjiQjif5JzgwpihRnhqR+LcOIXEuUHC+gD46ArqEbaW2ge8i5IxvIc14rSNDDh3vvOedecjj8X36i2HyiaDgcBL23504o01zrVOEnL/68lszptJ2+67meIz78aodqfrmcjOeZHsrR6u87yT6e11v9nhChF5z98ps3Wa2Ly+k0iQZyuPnvL90sv/n9Q7c7iJPBqPrTibgs7u6W4zwULBQ3j3v5qfLubcv1Rjob6drxKV+ud7t5qoTb973Vv19ls2rZHSZUPpFp/eWpuLjZFlOZMIx0PxM/s8s94evJVGYNw4vNBLzXFc4Tr3zH+eKJ97umXjXfl019Pkmy+vFTf0X9Mf8BHP273Mf8JK09c51wB/yaeNWGPl9l49GBr7fbmQLvB4d64lxI4tDPic+1FK4zIH/glmX8gTeeOfw7TgB9e9ky/bnM8gNf3KC/pnwGjPq3ziyryzj0xSn0g+92Uy0xf2Dmt0x+0JfHufG3LoqZToTrhd7qn3t54CI58BJb3hTbqdEXen8c9mN1udA55tdfnLDl+suXmdbCdSmfvWyWLastOHiia0cn/mVxeztXmN8L2PXjPeVndVwhtfH3mpH+Raq5Z/Z7j/623RVc5uNE1n4+RX2xnaXK9xzfw35GrWq50vEE8o3TxlGJX2xut7NMoT5w0V/Cv91m3+UL/3OFfJzn/KGPc8Mb8E/3NQPHfhjd1yeTP6f8df1liTj2R/kFfIX8mxXL5A+uwDEfHPmQP3CFehv3Qf3pfqm+IO4Sf4C/M5OvTMcJ9L1G/juTr+cQ/3rwT/pzzKf6zW43UajH/Vw/EreIQ1+c1uB/uaH9Q59D+ZI+c19mP8Tn0If7Yub5edjrdpXy4SqbJLp+VGIL0pcqBv3ovzf3jXy10X9E90c8Ya7Zn6kvt11w3H/juET5FnTffbpv+G9W30EfPT+kH/rWt5ivRL8TsM+036pdxn6gL87O8XwujD70d+h+76OPFavc4nyk00g2X70JJ1dXV7lSofDDILtbhJ7T6zM/iGK8qyL6mr94GJm3V/jDL75vIqD4Ig=="

func aviationAlarmWave(pulses int)([]byte,error){
 if pulses!=3&&pulses!=8{return nil,fmt.Errorf("invalid pulse count %d",pulses)}
 compressed,err:=base64.StdEncoding.DecodeString(originalAlarmPulseZlib64)
 if err!=nil{return nil,err}
 zr,err:=zlib.NewReader(bytes.NewReader(compressed))
 if err!=nil{return nil,err}
 pulse,err:=io.ReadAll(io.LimitReader(zr,3000))
 _=zr.Close()
 if err!=nil||len(pulse)!=1520{return nil,fmt.Errorf("corrupt embedded alarm sample: %v",err)}
 const rate=8000
 const interval=2480 // 310 ms
 sampleCount:=(pulses-1)*interval+len(pulse)+400
 data:=make([]byte,sampleCount)
 for i:=range data{data[i]=128} // PCM 8-bit silence, not zero
 for n:=0;n<pulses;n++{
  // The supplied pulse begins with a tiny silence and decays before its end.
  copy(data[n*interval:],pulse)
 }
 out:=make([]byte,44+len(data))
 copy(out[:4],"RIFF")
 binary.LittleEndian.PutUint32(out[4:8],uint32(len(out)-8))
 copy(out[8:12],"WAVE")
 copy(out[12:16],"fmt ")
 binary.LittleEndian.PutUint32(out[16:20],16)
 binary.LittleEndian.PutUint16(out[20:22],1)
 binary.LittleEndian.PutUint16(out[22:24],1)
 binary.LittleEndian.PutUint32(out[24:28],rate)
 binary.LittleEndian.PutUint32(out[28:32],rate)
 binary.LittleEndian.PutUint16(out[32:34],1)
 binary.LittleEndian.PutUint16(out[34:36],8)
 copy(out[36:40],"data")
 binary.LittleEndian.PutUint32(out[40:44],uint32(len(data)))
 copy(out[44:],data)
 return out,nil
}
