import { useEffect, useMemo, useState } from "react"
import { Spin, Empty, Typography, Space, Alert, Segmented, Tag, Tooltip, Button } from "antd"
import { BarChartOutlined } from "@ant-design/icons"
import { queryRangePromMetrics } from '../../../api/other'
import { EventMetricChart } from '../../chart/eventMetricChart'
import { TableWithPagination } from '../../../utils/TableWithPagination'
import { HandleShowTotal } from '../../../utils/lib'

const { Title, Text } = Typography

interface RangeResultItem {
    metric: Record<string, string>
    values: [number, string][]
}

interface SearchViewMetricsProps {
    datasourceType: string
    datasourceId: string[]
    promQL: string
}

export const SearchViewMetrics = ({ datasourceType, datasourceId, promQL }: SearchViewMetricsProps) => {
    const [chartData, setChartData] = useState<any[]>([])
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [view, setView] = useState<'chart' | 'table'>('chart')
    const [pagination, setPagination] = useState({ index: 1, size: 10, total: 0 })

    useEffect(() => {
        const fetchMetrics = async () => {
            try {
                setLoading(true)
                setError(null)

                const now = Math.floor(Date.now() / 1000)
                const params = {
                    datasourceIds: datasourceId.join(","),
                    query: promQL,
                    startTime: now - 600,
                    endTime: now,
                    step: 10,
                }

                const res = await queryRangePromMetrics(params)

                if (res.code !== 200) {
                    throw new Error(res.msg || "请求失败")
                }

                setChartData(res.data)
                setPagination((p) => ({ ...p, index: 1, total: computeRows(res.data).length }))
            } catch (err) {
                setError(err instanceof Error ? err.message : "网络错误")
                console.error("Fetch error:", err)
            } finally {
                setLoading(false)
            }
        }

        if (datasourceId.length > 0 && promQL) {
            fetchMetrics()
        }
    }, [datasourceId, promQL])

    const formatTimestamp = (timestamp: number) => {
        return new Date(timestamp * 1000).toLocaleString("zh-CN")
    }

    const computeRows = useMemo(() => {
        return (data: any[]) => {
            const results = (data || [])
                .filter((item: any) => item.status === 'success' && item.data?.result?.length > 0)
                .flatMap((item: any) => item.data.result as any[])
            return results.map((item: any, i: number) => {
                const values: [number, string][] = item.values || []
                const nums = values.map((v) => parseFloat(v[1])).filter((n) => !Number.isNaN(n))
                const latestVal = nums.length ? nums[nums.length - 1] : null
                const min = nums.length ? Math.min(...nums) : null
                const max = nums.length ? Math.max(...nums) : null
                const avg = nums.length ? Number((nums.reduce((a, b) => a + b, 0) / nums.length).toFixed(4)) : null
                const latestTs = values.length ? values[values.length - 1][0] : 0
                const name = item.metric?.__name__ || 'metric'
                const labelsEntries = Object.entries(item.metric).filter(([k]) => k !== '__name__')
                const labelsStr = labelsEntries.map(([k, v]) => `${k}=${v}`).join(', ')
                return {
                    id: `${name}-${i}-${latestTs}`,
                    metric: name,
                    labelsEntries,
                    labelsStr,
                    latest: latestVal,
                    min,
                    avg,
                    max,
                    points: nums.length,
                    latestTime: formatTimestamp(latestTs),
                }
            })
        }
    }, [])

    const rows = useMemo(() => computeRows(chartData), [chartData, computeRows])
    const pageRows = useMemo(() => {
        const start = (pagination.index - 1) * pagination.size
        const end = start + pagination.size
        return rows.slice(start, end)
    }, [rows, pagination])

    const columns = [
        {
            title: 'Metric',
            dataIndex: 'metric',
            key: 'metric',
            render: (text: string) => <Tag color="geekblue">{text}</Tag>,
        },
        {
            title: 'Labels',
            dataIndex: 'labelsEntries',
            key: 'labels',
            render: (entries: [string, string][], record: any) => (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, alignItems: 'center' }}>
                    {entries.map(([k, v]) => (
                        <Tag key={`${k}-${v}`} color="volcano" style={{ margin: 0 }}>{k}={v}</Tag>
                    ))}
                    <Tooltip title="复制全部标签">
                        <Button size="small" onClick={() => navigator.clipboard.writeText(record.labelsStr)}>复制</Button>
                    </Tooltip>
                </div>
            ),
        },
        { title: 'Latest', dataIndex: 'latest', key: 'latest' },
        { title: 'Min', dataIndex: 'min', key: 'min' },
        { title: 'Avg', dataIndex: 'avg', key: 'avg' },
        { title: 'Max', dataIndex: 'max', key: 'max' },
        { title: 'Points', dataIndex: 'points', key: 'points' },
        { title: 'Latest Time', dataIndex: 'latestTime', key: 'latestTime' },
    ]

    const onPageChange = (page: number) => setPagination((p) => ({ ...p, index: page }))
    const onPageSizeChange = (_: number, size: number) => setPagination({ index: 1, size, total: rows.length })

    if (loading) {
        return (
            <div
                style={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                    justifyContent: "center",
                    minHeight: "400px",
                    background: "#fafafa",
                    borderRadius: "8px",
                }}
            >
                <Spin size="large" />
                <div style={{ marginTop: "16px", textAlign: "center" }}>
                    <Text type="secondary" style={{ fontSize: "14px" }}>
                        正在获取最新的 Metric 数据
                    </Text>
                </div>
            </div>
        )
    }

    if (error) {
        return <Alert message="查询失败" description={error} type="error" showIcon style={{ margin: "20px 0" }} />
    }

    if (!chartData || chartData.length === 0) {
        return (
            <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                    <div>
                        <Text type="secondary" style={{ fontSize: "14px" }}>
                            当前查询条件下没有找到相关的 Metric 数据
                        </Text>
                    </div>
                }
                style={{
                    padding: "60px 20px",
                    background: "#fafafa",
                    borderRadius: "8px",
                    margin: "20px 0",
                }}
            />
        )
    }

    return (
        <div style={{ height: "100%", display: 'flex', flexDirection: 'column' }}>
            {/* Header */}
            <div
                style={{
                    padding: "16px 20px",
                    borderBottom: "1px solid #f0f0f0",
                    background: "linear-gradient(135deg, rgb(0 0 0) 0%, rgb(191 191 191) 100%)",
                    borderRadius: "8px 8px 0 0",
                    flexShrink: 0,
                }}
            >
                <Space align="center">
                    <BarChartOutlined style={{ fontSize: "20px", color: "white" }} />
                    <Title level={4} style={{ margin: 0, color: "white" }}>
                        {datasourceType}
                    </Title>
                </Space>
                <div style={{ float: 'right' }}>
                    <Segmented
                        options={[{ label: '折线图', value: 'chart' }, { label: '表格', value: 'table' }]}
                        value={view}
                        onChange={(val) => setView(val as 'chart' | 'table')}
                        size="small"
                        style={{ background: 'rgba(255,255,255,0.2)', color: '#fff', borderRadius: 6 }}
                    />
                </div>
            </div>

            {view === 'chart' ? (
                <div style={{ padding: '16px', flex: 1, display: 'flex' }}>
                    <EventMetricChart data={chartData} height={"100%"} />
                    <style>{`
                      .recharts-brush-texts { display: none; }
                      .recharts-brush .recharts-brush-traveller { fill: #000; stroke: #000; opacity: 0.8; }
                      .recharts-brush .recharts-brush-slide { fill: #f5f5f5; }
                    `}</style>
                </div>
            ) : (
                <div style={{ padding: '16px' }}>
                    <TableWithPagination
                        columns={columns}
                        dataSource={pageRows}
                        pagination={pagination}
                        onPageChange={onPageChange}
                        onPageSizeChange={onPageSizeChange}
                        scrollY={520}
                        rowKey={(r) => r.id}
                        showTotal={HandleShowTotal}
                        loading={loading}
                        locale={{}}
                        size="small"
                        sticky={true}
                    />
                </div>
            )}
        </div>
    )
}
