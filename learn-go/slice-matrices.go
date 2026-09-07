package main

import (
	"golang.org/x/tour/pic"
)

func Pic(dx, dy int) [][]uint8 {
	data := make([]uint8, dx*dy)
	img_matrix := make([][]uint8, dy)
	for i := 0; i < dx*dy; i++ {
		img_matrix[i % dy] = append(img_matrix[i % dy], data[i])
	}
	return img_matrix
}

func main() {
	pic.Show(Pic)
}
