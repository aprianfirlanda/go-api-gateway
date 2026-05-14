import type { ReactNode } from 'react'

export type Column<T> = {
  key: keyof T | string
  header: string
  render?: (row: T) => ReactNode
}

type TableProps<T> = {
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T, index: number) => string
}

export function Table<T>({ columns, rows, rowKey }: TableProps<T>) {
  return (
    <table className="ui-table">
      <thead>
        <tr>
          {columns.map((column) => (
            <th key={String(column.key)}>{column.header}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, index) => (
          <tr key={rowKey(row, index)}>
            {columns.map((column) => (
              <td key={String(column.key)}>
                {column.render ? column.render(row) : String((row as Record<string, unknown>)[String(column.key)] ?? '')}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}
