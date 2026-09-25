import { createContext, useContext, type ReactNode } from 'react'
import type { DataSource } from './ports'
import { mockDataSource } from './mock'

const DataSourceContext = createContext<DataSource>(mockDataSource)

/**
 * Elige la implementación de los puertos. En esta etapa solo existe `mock`; en la de cableado se agrega `gateway`
 * y se elige por `VITE_DATA_SOURCE`.
 */
export function DataSourceProvider({
  children,
  source = mockDataSource,
}: {
  children: ReactNode
  source?: DataSource
}) {
  return <DataSourceContext.Provider value={source}>{children}</DataSourceContext.Provider>
}

export function useDataSource(): DataSource {
  return useContext(DataSourceContext)
}

export const isMockDataSource = (import.meta.env.VITE_DATA_SOURCE ?? 'mock') === 'mock'
