package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func copyBody(reader *bufio.Reader, writer io.Writer, resp Response) error {
	switch resp.bodyFraming {

	case FramingNone:
		return nil

	case FramingContentLength:
		if resp.contentLength == 0 {
			return nil
		}

		limitReader := io.LimitReader(reader, int64(resp.contentLength))
		_, err := io.Copy(writer, limitReader)
		if err != nil {
			return fmt.Errorf("读取数据失败: %w", err)
		}
		return nil

	case FramingChunked:
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("读取数据失败: %w", err)
			}

			writer.Write([]byte(line))

			line = strings.TrimRight(line, "\r\n")
			line, _, _ = strings.Cut(line, ";")
			length, err := strconv.ParseInt(line, 16, 64)
			if err != nil {
				return fmt.Errorf("无效的块长度数据: %q", line)
			}

			if length == 0 {
				// 一直读，读到空行（\r\n）为止
				for {
					trailerLine, err := reader.ReadString('\n')
					if err != nil {
						return err
					}

					writer.Write([]byte(trailerLine))
					// 如果是空行，整个 Chunked 彻底结束
					if trailerLine == "\r\n" || trailerLine == "\n" {
						return nil
					}
				}
			}
			//转发数据
			_, err = io.CopyN(writer, reader, length)
			if err != nil {
				return fmt.Errorf("转发数据失败: %w", err)
			}
			//转发数据末尾的 CRLF
			var crlf [2]byte
			_, err = io.ReadFull(reader, crlf[:])
			if err != nil {
				return fmt.Errorf("读取数据失败: %w", err)
			}
			writer.Write(crlf[:])
		}
	case FramingUntilEOF:
		_, err := io.Copy(writer, reader)
		if err != nil {
			return fmt.Errorf("读取数据失败: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("未知的body framing: %d", resp.bodyFraming)
	}
}
