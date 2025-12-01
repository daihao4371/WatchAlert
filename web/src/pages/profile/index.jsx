import { Form, Input, Button, message, Card, Space, Typography } from "antd";
import React, { useEffect, useState } from "react";
import { getUserInfo, updateUser } from "../../api/user";
import { EditOutlined, SaveOutlined, CloseOutlined } from "@ant-design/icons";

const { Title } = Typography;

export default function Profile() {
    const [userInfo, setUserInfo] = useState(null);
    const [editingSection, setEditingSection] = useState(null); // 'basic', 'security' 或 null
    const [basicForm] = Form.useForm();
    const [securityForm] = Form.useForm();

    useEffect(() => {
        fetchUserInfo();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const fetchUserInfo = async () => {
        try {
            const res = await getUserInfo();
            setUserInfo(res.data);
            basicForm.setFieldsValue({
                username: res.data.username,
                email: res.data.email,
            });
            securityForm.setFieldsValue({
                password: ".........",
            });
        } catch (error) {
            console.error(error);
            message.error("获取用户信息失败");
        }
    };

    const handleBasicUpdate = async (values) => {
        try {
            const params = {
                userid: userInfo.userid,
                username: userInfo.username,
                email: values.email,
                phone: userInfo.phone || '',
                password: '', // 明确设置为空字符串，不更新密码
                role: userInfo.role,
                create_by: userInfo.create_by || '',
                create_at: userInfo.create_at || 0,
                joinDuty: userInfo.joinDuty || '',
                dutyUserId: userInfo.dutyUserId || '',
                tenants: userInfo.tenants || [],
            };
            await updateUser(params);
            setEditingSection(null);
            await fetchUserInfo();
            message.success("基本信息更新成功");
        } catch (error) {
            console.error(error);
            message.error("更新失败");
        }
    };

    const handleSecurityUpdate = async (values) => {
        try {
            const params = {
                ...userInfo,
                password: values.password,
            };
            await updateUser(params);
            setEditingSection(null);
            securityForm.setFieldsValue({ password: "........." });
            await fetchUserInfo();
            message.success("密码更新成功");
        } catch (error) {
            console.error(error);
            message.error("更新失败");
        }
    };

    const renderSectionHeader = (title, sectionKey) => (
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
            <Title level={4} style={{ margin: 0, fontWeight: 600 }}>{title}</Title>
            {editingSection !== sectionKey ? (
                <Button
                    type="link"
                    icon={<EditOutlined />}
                    onClick={() => {
                        setEditingSection(sectionKey);
                        if (sectionKey === 'security') {
                            securityForm.setFieldsValue({ password: '' });
                        }
                    }}
                    style={{ color: '#1890ff', padding: 0 }}
                >
                    编辑
                </Button>
            ) : (
                <Space>
                    <Button
                        type="link"
                        icon={<CloseOutlined />}
                        onClick={() => {
                            setEditingSection(null);
                            fetchUserInfo();
                        }}
                        style={{ padding: 0 }}
                    >
                        取消
                    </Button>
                    <Button
                        type="link"
                        icon={<SaveOutlined />}
                        onClick={() => {
                            if (sectionKey === 'basic') {
                                basicForm.submit();
                            } else if (sectionKey === 'security') {
                                securityForm.submit();
                            }
                        }}
                        style={{ color: '#1890ff', padding: 0 }}
                    >
                        保存
                    </Button>
                </Space>
            )}
        </div>
    );

    return (
        <div style={{ padding: '24px' }}>
            <Space direction="vertical" size="large" style={{ width: '100%' }}>
                {/* 基本信息 */}
                <Card 
                    style={{ 
                        boxShadow: '0 2px 8px rgba(0, 0, 0, 0.06)',
                        borderRadius: '8px'
                    }}
                >
                    <Form
                        form={basicForm}
                        onFinish={handleBasicUpdate}
                        layout="vertical"
                    >
                        {renderSectionHeader('基本信息', 'basic')}
                        <Form.Item
                            label="用户名"
                            name="username"
                            style={{ marginBottom: 16 }}
                        >
                            <Input disabled style={{ background: '#f5f5f5', cursor: 'not-allowed' }} />
                        </Form.Item>
                        <Form.Item
                            label="邮箱"
                            name="email"
                            rules={[
                                { type: "email", message: "请输入有效的邮箱地址" },
                                { required: editingSection === 'basic', message: "请输入邮箱" },
                            ]}
                            style={{ marginBottom: 0 }}
                        >
                            <Input
                                disabled={editingSection !== 'basic'}
                                placeholder="请输入邮箱地址"
                            />
                        </Form.Item>
                    </Form>
                </Card>

                {/* 安全 */}
                <Card 
                    style={{ 
                        boxShadow: '0 2px 8px rgba(0, 0, 0, 0.06)',
                        borderRadius: '8px'
                    }}
                >
                    <Form
                        form={securityForm}
                        onFinish={handleSecurityUpdate}
                        layout="vertical"
                    >
                        {renderSectionHeader('安全', 'security')}
                        <Form.Item
                            label="密码"
                            name="password"
                            rules={[
                                { min: 6, message: "密码至少6个字符" },
                                { required: editingSection === 'security', message: "请输入新密码" },
                            ]}
                            style={{ marginBottom: 0 }}
                        >
                            <Input.Password
                                disabled={editingSection !== 'security'}
                                placeholder={editingSection === 'security' ? "请输入新密码" : "........."}
                            />
                        </Form.Item>
                    </Form>
                </Card>
            </Space>
        </div>
    );
}