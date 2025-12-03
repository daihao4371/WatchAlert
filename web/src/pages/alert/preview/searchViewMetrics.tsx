import React, { useEffect, useState } from "react"
import { Spin, Tag, Empty, Card, Typography, Space, Divider, Row, Col, Alert, Tabs, Select } from "antd"
import {
    ClockCircleOutlined,
    TagsOutlined,
    BarChartOutlined,
    FileTextOutlined,
    LineChartOutlined,
} from "@ant-design/icons"
import {queryPromMetrics, queryRangePromMetrics, getPromLabelValues} from '../../../api/other'
import ReactECharts from "echarts-for-react"

const { Title, Text } = Typography

interface MetricItem {
    metric: Record<string, string>
    value: [number, string]
}

interface TimeSeriesData {
    metric: Record<string, string>
    values: [number, string][]
}

interface SearchViewMetricsProps {
    datasourceType: string
    datasourceId: string[]
    promQL: string
    variables?: Record<string, string> // 可选的变量映射，用于替换查询语句中的 $variable
}

export const SearchViewMetrics = ({ datasourceType, datasourceId, promQL, variables }: SearchViewMetricsProps) => {
    const [metrics, setMetrics] = useState<MetricItem[]>([])
    const [timeSeriesData, setTimeSeriesData] = useState<TimeSeriesData[]>([])
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)
    
    // 变量选择器相关状态
    const [instanceOptions, setInstanceOptions] = useState<string[]>([])
    const [ifNameOptions, setIfNameOptions] = useState<string[]>([])
    const [selectedInstance, setSelectedInstance] = useState<string | undefined>(variables?.instance)
    const [selectedIfName, setSelectedIfName] = useState<string | undefined>(variables?.ifName)
    const [loadingOptions, setLoadingOptions] = useState(false)
    
    // 检测查询语句中是否包含变量
    const hasInstanceVar = promQL.includes('$instance')
    const hasIfNameVar = promQL.includes('$ifName')
    
    // 从查询语句中提取 metric 名称（用于获取 label 值）
    const extractMetricName = (query: string): string => {
        // 尝试匹配常见的 metric 名称模式
        const metricPatterns = [
            /(ifHCIn\w+|ifIn\w+|ifOut\w+|ifHCOut\w+)/,
            /(\w+)\{/,
        ]
        for (const pattern of metricPatterns) {
            const match = query.match(pattern)
            if (match && match[1]) {
                return match[1]
            }
        }
        return ''
    }

    // 获取 label 值的函数
    useEffect(() => {
        const fetchLabelValues = async () => {
            if (datasourceId.length === 0) return
            
            const metricName = extractMetricName(promQL)
            const firstDatasourceId = datasourceId[0]
            
            setLoadingOptions(true)
            try {
                const promises: Promise<any>[] = []
                
                // 如果需要 instance，获取 instance 值
                if (hasInstanceVar) {
                    promises.push(
                        getPromLabelValues({
                            datasourceId: firstDatasourceId,
                            labelName: 'instance',
                            metricName: metricName || undefined
                        }).then(res => {
                            if (res.code === 200 && Array.isArray(res.data)) {
                                setInstanceOptions(res.data)
                                // 如果没有选中值且有可用选项，自动选择第一个
                                if (!selectedInstance && res.data.length > 0) {
                                    setSelectedInstance(res.data[0])
                                }
                            }
                        }).catch(err => {
                            console.error('获取 instance 值失败:', err)
                        })
                    )
                }
                
                // 如果需要 ifName，获取 ifName 值
                if (hasIfNameVar) {
                    promises.push(
                        getPromLabelValues({
                            datasourceId: firstDatasourceId,
                            labelName: 'ifName',
                            metricName: metricName || undefined
                        }).then(res => {
                            if (res.code === 200 && Array.isArray(res.data)) {
                                setIfNameOptions(res.data)
                                // 如果没有选中值且有可用选项，自动选择第一个
                                if (!selectedIfName && res.data.length > 0) {
                                    setSelectedIfName(res.data[0])
                                }
                            }
                        }).catch(err => {
                            console.error('获取 ifName 值失败:', err)
                        })
                    )
                }
                
                await Promise.all(promises)
            } catch (err) {
                console.error('获取 label 值失败:', err)
            } finally {
                setLoadingOptions(false)
            }
        }
        
        // 如果查询包含变量，获取可用的 label 值
        if ((hasInstanceVar || hasIfNameVar) && datasourceId.length > 0 && promQL) {
            fetchLabelValues()
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [datasourceId, promQL, hasInstanceVar, hasIfNameVar])
    
    useEffect(() => {
        const fetchMetrics = async () => {
            try {
                setLoading(true)
                setError(null)
                // 关键修复：清空旧数据,防止显示上一次的缓存数据
                setMetrics([])
                setTimeSeriesData([])

                // 并行请求: 1) 即时数据 (用于列表视图), 2) 时间序列数据 (用于图表视图)
                const now = Math.floor(Date.now() / 1000)
                const oneHourAgo = now - 3600 // 过去1小时

                // 构建查询参数，包含变量
                const queryParams: any = {
                    datasourceIds: datasourceId.join(","),
                    query: promQL,
                }
                
                // 合并外部传入的变量和用户选择的变量
                const mergedVariables: Record<string, string> = { ...variables }
                if (selectedInstance) {
                    mergedVariables.instance = selectedInstance
                }
                if (selectedIfName) {
                    mergedVariables.ifName = selectedIfName
                }
                
                // 如果有变量，添加到查询参数中
                if (Object.keys(mergedVariables).length > 0) {
                    // 方式1: 使用 variables[key]=value 格式
                    Object.keys(mergedVariables).forEach(key => {
                        queryParams[`variables[${key}]`] = mergedVariables[key]
                    })
                    // 方式2: 同时传递直接的 instance 和 ifName 参数（兼容性）
                    if (mergedVariables.instance) {
                        queryParams.instance = mergedVariables.instance
                    }
                    if (mergedVariables.ifName) {
                        queryParams.ifName = mergedVariables.ifName
                    }
                }

                const [instantRes, rangeRes] = await Promise.all([
                    queryPromMetrics(queryParams),
                    queryRangePromMetrics({
                        ...queryParams,
                        start: oneHourAgo,
                        end: now,
                        step: 60, // 每分钟一个数据点
                    })
                ])

                // 处理即时数据
                if (instantRes.code === 200) {
                    const allResults = instantRes.data
                        .filter((item) => item.status === "success" && item.data?.result?.length > 0)
                        .flatMap((item) => item.data.result)
                    setMetrics(allResults)
                }

                // 处理时间序列数据
                if (rangeRes.code === 200) {
                    const timeSeriesResults = rangeRes.data
                        .filter((item) => item.status === "success" && item.data?.result?.length > 0)
                        .flatMap((item) => item.data.result)
                    setTimeSeriesData(timeSeriesResults)
                } else {
                    throw new Error(rangeRes.msg || "请求失败")
                }
            } catch (err) {
                setError(err instanceof Error ? err.message : "网络错误")
                console.error("Fetch error:", err)
            } finally {
                setLoading(false)
            }
        }

        // 关键修复：只有当必要参数都存在时才发起请求
        // 如果查询包含变量，需要等待变量值被选择
        const shouldFetch = datasourceId.length > 0 && promQL && promQL.trim() !== ''
        const hasRequiredVariables = (!hasInstanceVar || selectedInstance) && (!hasIfNameVar || selectedIfName)
        
        if (shouldFetch && hasRequiredVariables) {
            fetchMetrics()
        } else {
            // 如果参数不完整,清空数据并停止加载状态
            setMetrics([])
            setTimeSeriesData([])
            setLoading(false)
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [datasourceId, promQL, datasourceType, selectedInstance, selectedIfName, hasInstanceVar, hasIfNameVar, variables])

    const formatTimestamp = (timestamp: number) => {
        return new Date(timestamp * 1000).toLocaleString("zh-CN")
    }

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

    if (metrics.length === 0) {
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

    // 准备时间序列图表数据
    const getChartOption = () => {
        if (!timeSeriesData || timeSeriesData.length === 0) {
            return {}
        }

        // 为每个时间序列生成系列数据
        const series = timeSeriesData.map((item, index) => {
            // 生成系列名称
            const metricKeys = Object.keys(item.metric).filter((key) => key !== "__name__")
            let seriesName = item.metric.__name__ || `Metric #${index + 1}`

            // 优先使用常见的标签名称
            const preferredKeys = ['instance', 'job', 'node', 'pod', 'container', 'service']
            for (const key of preferredKeys) {
                if (item.metric[key]) {
                    seriesName = `${item.metric[key]}`
                    break
                }
            }

            // 处理时间序列数据点
            const data = item.values.map(([timestamp, value]) => {
                return [timestamp * 1000, Number.parseFloat(value)]
            })

            return {
                name: seriesName,
                type: 'line',
                smooth: true, // 平滑曲线
                showSymbol: false, // 不显示数据点标记(数据点多时更清晰)
                data: data,
                lineStyle: {
                    width: 2
                },
                emphasis: {
                    focus: 'series'
                },
                metricInfo: item.metric // 保存完整的 metric 信息用于 tooltip
            }
        })

        // 生成颜色列表
        const colors = [
            '#5470c6', '#91cc75', '#fac858', '#ee6666', '#73c0de',
            '#3ba272', '#fc8452', '#9a60b4', '#ea7ccc', '#5470c6'
        ]

        return {
            color: colors,
            tooltip: {
                trigger: 'axis',
                axisPointer: {
                    type: 'line',
                    lineStyle: {
                        type: 'dashed'
                    }
                },
                backgroundColor: 'rgba(255, 255, 255, 0.95)',
                borderColor: '#ddd',
                borderWidth: 1,
                textStyle: {
                    color: '#333'
                },
                formatter: (params: any) => {
                    if (!params || params.length === 0) return ''

                    const time = new Date(params[0].value[0]).toLocaleString('zh-CN', {
                        year: 'numeric',
                        month: '2-digit',
                        day: '2-digit',
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit'
                    })

                    let tooltip = `<div style="padding: 4px;">
                        <div style="font-weight: bold; margin-bottom: 8px;">${time}</div>
                    `

                    params.forEach((param: any) => {
                        const value = param.value[1]
                        const formattedValue = typeof value === 'number' ? value.toFixed(2) : value
                        tooltip += `
                            <div style="display: flex; align-items: center; margin-bottom: 4px;">
                                <span style="display: inline-block; width: 10px; height: 10px; border-radius: 50%; background: ${param.color}; margin-right: 8px;"></span>
                                <span style="font-weight: 500;">${param.seriesName}:</span>
                                <span style="margin-left: 8px; font-weight: bold;">${formattedValue}</span>
                            </div>
                        `
                    })

                    tooltip += '</div>'
                    return tooltip
                }
            },
            legend: {
                type: 'scroll',
                bottom: 0,
                data: series.map(s => s.name),
                textStyle: {
                    fontSize: 12
                }
            },
            grid: {
                left: '3%',
                right: '4%',
                bottom: timeSeriesData.length > 5 ? '12%' : '8%',
                top: '5%',
                containLabel: true
            },
            xAxis: {
                type: 'time',
                boundaryGap: false,
                axisLabel: {
                    fontSize: 11,
                    formatter: (value: number) => {
                        const date = new Date(value)
                        return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`
                    }
                },
                splitLine: {
                    show: true,
                    lineStyle: {
                        type: 'dashed',
                        color: '#f0f0f0'
                    }
                }
            },
            yAxis: {
                type: 'value',
                axisLabel: {
                    fontSize: 11,
                    formatter: (value: number) => {
                        // 格式化数字
                        if (value >= 1000000) {
                            return (value / 1000000).toFixed(1) + 'M'
                        }
                        if (value >= 1000) {
                            return (value / 1000).toFixed(1) + 'K'
                        }
                        if (value < 1 && value > 0) {
                            return value.toFixed(2)
                        }
                        return value.toFixed(0)
                    }
                },
                splitLine: {
                    show: true,
                    lineStyle: {
                        type: 'dashed',
                        color: '#f0f0f0'
                    }
                }
            },
            series: series
        }
    }

    // 渲染列表视图
    const renderListView = () => (
        <Space direction="vertical" size="middle" style={{ width: "100%", marginTop: "10px" }}>
            {metrics.map((item, index) => {
                const metricKeys = Object.keys(item.metric).filter((key) => key !== "__name__")

                return (
                    <Card
                        key={index}
                        hoverable
                        style={{
                            borderLeft: `4px solid #1890ff`,
                            boxShadow: "0 2px 8px rgba(0,0,0,0.06)",
                        }}
                    >
                        <Row gutter={[16, 16]}>
                            {/* 左侧:Metric 信息 */}
                            <Col span={16}>
                                <Space direction="vertical" size="small" style={{ width: "100%" }}>
                                    {/* 标题 */}
                                    <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                                        <BarChartOutlined style={{ color: "#1890ff", fontSize: "16px" }} />
                                        <Text strong style={{ fontSize: "16px" }}>
                                            Metric #{index + 1}
                                        </Text>
                                    </div>

                                    <Divider style={{ margin: "8px 0" }} />

                                    {/* 标签信息 */}
                                    {metricKeys.length > 0 && (
                                        <div>
                                            <div style={{ display: "flex", alignItems: "center", gap: "6px", marginBottom: "8px" }}>
                                                <TagsOutlined style={{ color: "#666", fontSize: "14px" }} />
                                                <Text type="secondary" style={{ fontSize: "12px", fontWeight: 500 }}>
                                                    标签信息
                                                </Text>
                                            </div>
                                            <div style={{ display: "flex", flexWrap: "wrap", gap: "6px" }}>
                                                {metricKeys.map((key) => (
                                                    <Tag key={key} color="blue" style={{ margin: 0 }}>
                                                        <Text style={{ fontSize: "12px" }}>
                                                            <span style={{ fontWeight: 600 }}>{key}:</span> {item.metric[key]}
                                                        </Text>
                                                    </Tag>
                                                ))}
                                            </div>
                                        </div>
                                    )}
                                </Space>
                            </Col>

                            {/* 右侧:数值和时间 */}
                            <Col span={8}>
                                <div
                                    style={{
                                        textAlign: "right",
                                        height: "100%",
                                        display: "flex",
                                        flexDirection: "column",
                                        justifyContent: "center",
                                    }}
                                >
                                    <div style={{ marginBottom: "8px" }}>
                                        <Text type="secondary" style={{ fontSize: "12px", display: "block" }}>
                                            数值
                                        </Text>
                                        <Text
                                            style={{
                                                fontSize: "24px",
                                                fontWeight: "bold",
                                                color: item.value[1] === "0" ? "#52c41a" : "#1890ff",
                                            }}
                                        >
                                            {Number.parseFloat(item.value[1]).toLocaleString()}
                                        </Text>
                                    </div>

                                    <div>
                                        <Text type="secondary" style={{ fontSize: "11px", display: "block" }}>
                                            <ClockCircleOutlined style={{ marginRight: "4px" }} />
                                            时间戳
                                        </Text>
                                        <Text style={{ fontSize: "12px", color: "#666" }}>{formatTimestamp(item.value[0])}</Text>
                                    </div>
                                </div>
                            </Col>
                        </Row>
                    </Card>
                )
            })}
        </Space>
    )

    // 渲染图表视图
    const renderChartView = () => (
        <Card
            style={{
                marginTop: "10px",
                boxShadow: "0 2px 8px rgba(0,0,0,0.06)"
            }}
        >
            <ReactECharts
                option={getChartOption()}
                style={{ height: '400px', width: '100%' }}
                opts={{ renderer: 'canvas' }}
            />

            {/* 数据统计信息 */}
            <Divider style={{ margin: "16px 0" }} />
            <Row gutter={16}>
                <Col span={6}>
                    <Card size="small" style={{ textAlign: 'center', background: '#f0f5ff' }}>
                        <div style={{ fontSize: '12px', color: '#666', marginBottom: '4px' }}>时间序列数</div>
                        <div style={{ fontSize: '20px', fontWeight: 'bold', color: '#1890ff' }}>
                            {timeSeriesData.length}
                        </div>
                    </Card>
                </Col>
                <Col span={6}>
                    <Card size="small" style={{ textAlign: 'center', background: '#f6ffed' }}>
                        <div style={{ fontSize: '12px', color: '#666', marginBottom: '4px' }}>数据点</div>
                        <div style={{ fontSize: '20px', fontWeight: 'bold', color: '#52c41a' }}>
                            {timeSeriesData.length > 0 ? timeSeriesData[0].values.length : 0}
                        </div>
                    </Card>
                </Col>
                <Col span={6}>
                    <Card size="small" style={{ textAlign: 'center', background: '#fff7e6' }}>
                        <div style={{ fontSize: '12px', color: '#666', marginBottom: '4px' }}>时间范围</div>
                        <div style={{ fontSize: '14px', fontWeight: 'bold', color: '#fa8c16' }}>
                            过去1小时
                        </div>
                    </Card>
                </Col>
                <Col span={6}>
                    <Card size="small" style={{ textAlign: 'center', background: '#fff1f0' }}>
                        <div style={{ fontSize: '12px', color: '#666', marginBottom: '4px' }}>采样间隔</div>
                        <div style={{ fontSize: '20px', fontWeight: 'bold', color: '#ff4d4f' }}>
                            1分钟
                        </div>
                    </Card>
                </Col>
            </Row>
        </Card>
    )

    return (
        <div style={{ minHeight: "500px" }}>
            {/* Header */}
            <div
                style={{
                    padding: "20px 24px",
                    borderBottom: "1px solid #f0f0f0",
                    background: "linear-gradient(135deg, rgb(0 0 0) 0%, rgb(191 191 191) 100%)",
                    borderRadius: "8px 8px 0 0",
                }}
            >
                <Space align="center" style={{ width: "100%", justifyContent: "space-between" }}>
                    <Space align="center">
                        <BarChartOutlined style={{ fontSize: "20px", color: "white" }} />
                        <Title level={4} style={{ margin: 0, color: "white" }}>
                            {datasourceType}
                        </Title>
                    </Space>
                    
                    {/* 变量选择器 */}
                    {(hasInstanceVar || hasIfNameVar) && (
                        <Space>
                            {hasInstanceVar && (
                                <Select
                                    style={{ minWidth: 200 }}
                                    placeholder="选择 instance"
                                    value={selectedInstance}
                                    onChange={(value) => setSelectedInstance(value)}
                                    loading={loadingOptions}
                                    options={instanceOptions.map(opt => ({ label: opt, value: opt }))}
                                    allowClear
                                />
                            )}
                            {hasIfNameVar && (
                                <Select
                                    style={{ minWidth: 200 }}
                                    placeholder="选择 ifName"
                                    value={selectedIfName}
                                    onChange={(value) => setSelectedIfName(value)}
                                    loading={loadingOptions}
                                    options={ifNameOptions.map(opt => ({ label: opt, value: opt }))}
                                    allowClear
                                />
                            )}
                        </Space>
                    )}
                </Space>
            </div>

            {/* Tabs 切换视图 */}
            <Tabs
                defaultActiveKey="chart"
                style={{ marginTop: "10px" }}
                items={[
                    {
                        key: 'chart',
                        label: (
                            <span>
                                <LineChartOutlined />
                                图表视图
                            </span>
                        ),
                        children: renderChartView()
                    },
                    {
                        key: 'list',
                        label: (
                            <span>
                                <FileTextOutlined />
                                列表视图
                            </span>
                        ),
                        children: renderListView()
                    }
                ]}
            />
        </div>
    )
}
