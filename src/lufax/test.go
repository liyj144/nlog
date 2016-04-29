package main
import (
"fmt"
"time"
)

func main() {
    fmt.Println("Hello, 世界")
      fmt.Println(time.Now())
        tm := time.Unix(time.Now().Unix(), 0)
          fmt.Println(tm.Format("20060102150405"))
}
