package main

import (
	"bufio"
	"fmt"
	"io"
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
		/*
			var crlf [2]byte
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("读取数据失败: %w", err)
				}

				line = strings.TrimRight(line, "\r\n")
				line, _, _ = strings.Cut(line, ";")
				length, err := strconv.ParseInt(line, 16, 64)
				if err != nil {
					return fmt.Errorf("无效的块长度数据: %q", line)
				}

				if length == 0 {
					_, err := io.ReadFull(reader, crlf[:])
					if err != nil {
						return fmt.Errorf("读取数据失败: %w", err)
					}
					break
				}
				dataReader := io.LimitReader(reader, length)
				_, err = io.Copy(writer, dataReader)
				if err != nil {
					return fmt.Errorf("读取数据失败: %w", err)
				}

				_, err = io.ReadFull(reader, crlf[:])
				if err != nil {
					return fmt.Errorf("读取数据失败: %w", err)
				}
			}
			return nil
		*/
		_, err := io.Copy(writer, reader)
		if err != nil {
			return fmt.Errorf("读取数据失败: %w", err)
		}
		return nil
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
