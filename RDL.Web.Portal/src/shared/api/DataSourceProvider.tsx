import { createContext, useContext, type ReactNode } from 'react'
import type { DataSource } from './ports'
import { config } from '@/shared/config'
import { mockDataSource } from './mock'

const DataSourceContext = createContext<DataSource>(mockDataSource)

/** Entrega la implementación de los puertos. La elige `src/app/sources.ts` según `VITE_DATA_SOURCE`. */
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

export const isMockDataSource = config.dataSource === 'mock'
