package executor

import (
	"fmt"
	"strings"
)

// FormatResult 把执行器结果转换成客户端文本
func FormatResult(result ResultSet) string {
	switch result := result.(type) {
	case CreateTableResult:
		return "CREATE TABLE " + result.TableName

	case InsertResult:
		return fmt.Sprintf("INSERT %d rows", result.Count)

	case UpdateResult:
		return fmt.Sprintf("UPDATE %d rows", result.Count)

	case DeleteResult:
		return fmt.Sprintf("DELETE %d rows", result.Count)

	case RowsResult:
		lines := make([]string, 0, len(result.Rows)+2)
		lines = append(lines, strings.Join(result.Columns, " | "))

		for _, row := range result.Rows {
			values := make([]string, len(row))
			for index, value := range row {
				values[index] = value.String()
			}
			lines = append(lines, strings.Join(values, " | "))
		}

		lines = append(
			lines,
			fmt.Sprintf("(%d rows)", len(result.Rows)),
		)
		return strings.Join(lines, "\n")

	default:
		return fmt.Sprintf("UNKNOWN RESULT %T", result)
	}
}
