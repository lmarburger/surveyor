package surveyor

import (
	"io"
	"log"
)

func ClosePrintErr(body io.Closer) {
	err := body.Close()
	if err != nil {
		log.Printf("error closing body: %v", err)
	}
}
