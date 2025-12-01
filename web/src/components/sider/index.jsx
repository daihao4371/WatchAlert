import React, {useEffect, useState} from 'react';
import {
    UserOutlined,
    BellOutlined,
    PieChartOutlined,
    NotificationOutlined,
    CalendarOutlined,
    DashboardOutlined,
    DeploymentUnitOutlined,
    AreaChartOutlined,
    FileDoneOutlined,
    SettingOutlined,
    ExceptionOutlined,
    ApiOutlined, TeamOutlined, DownOutlined, LogoutOutlined
} from '@ant-design/icons';
import {Link, useNavigate} from 'react-router-dom';
import {Menu, Layout, Typography, Dropdown, Space, message, Spin, theme, Popover, Avatar, Divider} from 'antd';
import logoIcon from "../../img/logo.svg";
import {getUserInfo} from "../../api/user";
import {getTenantList} from "../../api/tenant";

const { SubMenu } = Menu;
const { Sider } = Layout;

const adminMenuItems = [
    { key: '1', path: '/', icon: <AreaChartOutlined />, label: '概览' },
    {
        key: '2',
        icon: <BellOutlined />,
        label: '告警管理',
        children: [
            { key: '2-1', path: '/ruleGroup', label: '告警规则' },
            { key: '2-5', path: '/tmplType/Metrics/group', label: '规则模版' },
            { key: '2-6', path: '/subscribes', label: '告警订阅' }
        ]
    },
    { key: '12', path: '/faultCenter', icon: <ExceptionOutlined />, label: '故障中心' },
    {
        key: '3',
        icon: <NotificationOutlined />,
        label: '通知管理',
        children: [
            { key: '3-1', path: '/noticeObjects', label: '通知对象' },
            { key: '3-2', path: '/noticeTemplate', label: '通知模版' },
            { key: '3-3', path: '/noticeRecords', label: '通知记录' }
        ]
    },
    { key: '4', path: '/dutyManage', icon: <CalendarOutlined />, label: '值班中心' },
    {
        key: '11',
        icon: <ApiOutlined />,
        label: '网络分析',
        children: [
            { key: '11-1', path: '/probing', label: '拨测任务' },
            { key: '11-2', path: '/onceProbing', label: '及时拨测' }
        ]
    },
    { key: '6', path: '/datasource', icon: <PieChartOutlined />, label: '数据源' },
    { key: '8', path: '/folders', icon: <DashboardOutlined />, label: '仪表盘' },
    {
        key: '5',
        icon: <UserOutlined />,
        label: '人员组织',
        children: [
            { key: '5-1', path: '/user', label: '用户管理' },
            { key: '5-2', path: '/userRole', label: '角色管理' }
        ]
    },
    { key: '7', path: '/tenants', icon: <DeploymentUnitOutlined />, label: '租户管理' },
    { key: '9', path: '/auditLog', icon: <FileDoneOutlined />, label: '日志审计' },
    { key: '10', path: '/settings', icon: <SettingOutlined />, label: '系统设置' }
];

const userMenuItems = [
    { key: '1', path: '/', icon: <AreaChartOutlined />, label: '概览' },
    {
        key: '2',
        icon: <BellOutlined />,
        label: '告警管理',
        children: [
            { key: '2-1', path: '/ruleGroup', label: '告警规则' },
            { key: '2-5', path: '/tmplType/Metrics/group', label: '规则模版' },
            { key: '2-6', path: '/subscribes', label: '告警订阅' }
        ]
    },
    { key: '12', path: '/faultCenter', icon: <ExceptionOutlined />, label: '故障中心' },
    {
        key: '3',
        icon: <NotificationOutlined />,
        label: '通知管理',
        children: [
            { key: '3-1', path: '/noticeObjects', label: '通知对象' },
            { key: '3-2', path: '/noticeTemplate', label: '通知模版' },
            { key: '3-3', path: '/noticeRecords', label: '通知记录' }
        ]
    },
    {
        key: '4',
        icon: <CalendarOutlined />,
        label: '值班管理',
        children: [
            { key: '4-1', path: '/dutyManage', label: '值班日程' }
        ]
    },
    {
        key: '11',
        icon: <ApiOutlined />,
        label: '网络分析',
        children: [
            { key: '11-1', path: '/probing', label: '拨测任务' },
            { key: '11-2', path: '/onceProbing', label: '及时拨测' }
        ]
    },
    { key: '6', path: '/datasource', icon: <PieChartOutlined />, label: '数据源' },
    { key: '8', path: '/folders', icon: <DashboardOutlined />, label: '仪表盘' }
];

export const ComponentSider = () => {
    const navigate = useNavigate();
    const [selectedMenuKey, setSelectedMenuKey] = useState('');
    const [userInfo, setUserInfo] = useState(null)
    const [loading, setLoading] = useState(true)
    const [tenantList, setTenantList] = useState([])
    const [getTenantStatus, setTenantStatus] = useState(null)

    const {
        token: { colorBgContainer, borderRadiusLG },
    } = theme.useToken()

    const handleMenuClick = (info) => {
        const menuItem = findMenuItem(userInfo?.role === 'admin' ? adminMenuItems : userMenuItems, info.key);
        if (menuItem?.path) {
            setSelectedMenuKey(info.key);
            navigate(menuItem.path);
        }
    };

    const findMenuItem = (items, key) => {
        for (const item of items) {
            if (item.key === key) return item;
            if (item.children) {
                const found = item.children.find(child => child.key === key);
                if (found) return found;
            }
        }
        return null;
    };

    const convertMenuItems = (items) => {
        return items.map(item => {
            if (item.children) {
                return {
                    key: item.key,
                    icon: item.icon,
                    label: item.label,
                    children: item.children.map(child => ({
                        key: child.key,
                        label: child.label,
                    })),
                };
            }
            return {
                key: item.key,
                icon: item.icon,
                label: item.label,
            };
        });
    };

    const handleLogout = () => {
        localStorage.clear()
        navigate("/login")
    }

    const userPopoverMenuItems = [
        {
            key: "profile",
            icon: <UserOutlined />,
            label: <Link to="/profile">个人信息</Link>,
        },
        {
            type: 'divider',
        },
        {
            key: "logout",
            icon: <LogoutOutlined />,
            label: "退出登录",
            danger: true,
            onClick: handleLogout,
        },
    ]

    useEffect(() => {
        fetchUserInfo()
    }, [])

    const fetchUserInfo = async () => {
        try {
            const res = await getUserInfo()
            setUserInfo(res.data)

            if (res.data.userid) {
                await fetchTenantList(res.data.userid)
            }

            setLoading(false)
        } catch (error) {
            console.error("Failed to fetch user info:", error)
            window.localStorage.removeItem("Authorization")
            navigate("/login")
        }
    }

    const fetchTenantList = async (userid) => {
        try {
            const params = {
                userId: userid,
            }
            const res = await getTenantList(params)

            if (res.data === null || res.data.length === 0) {
                message.error("该用户没有可用租户")
                return
            }

            const opts = res.data.map((key, index) => ({
                label: key.name,
                value: key.id,
                index: index,
            }))

            setTenantList(opts)

            if (getTenantName() === null && opts.length > 0) {
                localStorage.setItem("TenantName", opts[0].label)
                localStorage.setItem("TenantID", opts[0].value)
                localStorage.setItem("TenantIndex", opts[0].index)
            }

            setTenantStatus(true)
        } catch (error) {
            console.error("Failed to fetch tenant list:", error)
            localStorage.clear()
            message.error("获取租户错误, 退出登录")
        }
    }

    const getTenantName = () => {
        return localStorage.getItem("TenantName")
    }

    const getTenantIndex = () => {
        return localStorage.getItem("TenantIndex")
    }

    const changeTenant = (c) => {
        localStorage.setItem("TenantIndex", c.key)
        if (c.item.props.name) {
            localStorage.setItem("TenantName", c.item.props.name)
        }
        if (c.item.props.value) {
            localStorage.setItem("TenantID", c.item.props.value)
        }

        setSelectedMenuKey('1')
        navigate('/')
        window.location.reload();
    }

    const tenantMenu = (
        <Menu selectable defaultSelectedKeys={[getTenantIndex()]} onSelect={changeTenant}>
            {tenantList.map((item) => (
                <Menu.Item key={item.index} name={item.label} value={item.value}>
                    {item.label}
                </Menu.Item>
            ))}
        </Menu>
    )

    if (loading || !getTenantStatus) {
        return (
            <div
                style={{
                    height: "100vh",
                    display: "flex",
                    justifyContent: "center",
                    alignItems: "center",
                    background: colorBgContainer,
                }}
            >
                <Spin size="large">
                    <div style={{ padding: '50px' }} />
                </Spin>
            </div>
        )
    }

    return (
        <Sider
            style={{
                overflow: 'hidden',
                height: '100%',
                background: '#000',
                borderRadius: '12px',
                display: 'flex',
                flexDirection: 'column',
                position: 'relative', 
            }}
            theme="dark"
        >
            {/* 顶部Logo和租户选择区域 */}
            <div style={{
                padding: '16px 16px 0',
                position: 'sticky',
                top: 0,
                zIndex: 1,
            }}>
                <div style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    marginBottom: 16,
                    marginTop: '-70px',
                }}>
                    <img
                        src={logoIcon || "/placeholder.svg"}
                        alt="WatchAlert Logo"
                        style={{ width: "160px", height: "140px", borderRadius: "8px" }}
                    />
                </div>

                <Dropdown menu={{ items: tenantList.map((item) => ({
                    key: item.index,
                    label: item.label,
                    onClick: () => changeTenant({ key: item.index, item: { props: { name: item.label, value: item.value } } })
                })) }} trigger={["click"]} placement="bottomLeft">
                    <div style={{
                        display: 'flex',
                        marginTop: '-40px',
                        alignItems: 'center',
                        padding: '8px 12px',
                        borderRadius: '4px',
                        cursor: 'pointer',
                        background: 'rgba(255, 255, 255, 0.1)',
                        marginBottom: '16px',
                        ':hover': {
                            background: 'rgba(255, 255, 255, 0.2)',
                        }
                    }}>
                        <TeamOutlined style={{color: '#fff', fontSize: '14px', marginRight: '8px'}}/>
                        <Typography.Text
                            style={{color: '#fff', fontSize: '14px', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis'}}>
                            {getTenantName()}
                        </Typography.Text>
                        <DownOutlined style={{color: '#fff', fontSize: '12px'}}/>
                    </div>
                </Dropdown>
            </div>

            <Divider style={{margin: '0', background: 'rgba(255, 255, 255, 0.1)'}}/>

            {/* 主内容，预留底部空间 */}
            <div
                style={{
                    textAlign:'left',
                    alignItems: 'flex-start',
                    overflowY: 'auto',
                    flex: 1,
                    height: '76vh',
                    paddingBottom: 70, // 预留底部空间
                }}
            >
                <Menu
                    theme="dark"
                    mode="inline"
                    selectedKeys={[selectedMenuKey]}
                    style={{ background: 'transparent'}}
                    items={convertMenuItems(userInfo?.role === 'admin' ? adminMenuItems : userMenuItems)}
                    onClick={handleMenuClick}
                />
            </div>

            {/* 绝对定位底部用户信息 */}
            <div style={{
                position: 'absolute',
                left: 0,
                bottom: 0,
                width: '100%',
                padding: '10px',
                borderTop: '1px solid rgba(255, 255, 255, 0.1)',
                background: '#000',
            }}>
                <Popover content={<Menu items={userPopoverMenuItems} mode="vertical" />} trigger="click" placement="topRight">
                    <div style={{
                        display: "flex",
                        alignItems: "center",
                        cursor: "pointer",
                        padding: '8px',
                        borderRadius: '4px',
                        width: '100%',
                    }}>
                        <Avatar
                            style={{
                                backgroundColor: "#1890ff",
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "center",
                            }}
                            size="default"
                            icon={<UserOutlined />}
                        />
                        <div style={{marginLeft: "12px", overflow: 'hidden'}}>
                            <Typography.Text style={{color: "#FFFFFFA6", display: 'block'}}>
                                {userInfo?.username || ""}
                            </Typography.Text>
                        </div>
                    </div>
                </Popover>
            </div>
        </Sider>
    );
};