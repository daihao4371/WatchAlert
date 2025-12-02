'use client'
import React, { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { useNavigate } from 'react-router-dom';
import './global.css';
import { checkUser, loginUser, registerUser, getOidcInfo } from '../api/user';
import { message } from "antd";
import { UserManager } from 'oidc-client';
import loginIllustration from '../img/login-illustration.png';

export const Login = () => {
    const [showOidcButtons,setShowOidcButtons] = useState(false);
    const [passwordModal, setPasswordModal] = useState(false);
    const [isModalVisible, setIsModalVisible] = useState(false);
    const navigate = useNavigate();

    // 检查是否已登录
    useEffect(() => {
        const token = localStorage.getItem('Authorization');
        if (token) {
            navigate('/');
        }
    }, [navigate]);

    // 检查 admin 用户是否存在
    useEffect(() => {
        const checkAdminUser = async () => {
            try {
                const params = { username: 'admin' };
                const res = await checkUser(params);
                if (res?.data?.username === 'admin') {
                    setPasswordModal(true);
                }
            } catch (error) {
                console.error(error);
            }
        };
        checkAdminUser();
    }, []);

    // 处理登录表单提交
    const onFinish = async (event) => {
        event.preventDefault();
        const formData = new FormData(event.target);
        const params = {
            username: formData.get('username'),
            password: formData.get('password'),
        };
        try {
            const response = await loginUser(params);
            if (response.data) {
                const info = response.data;
                localStorage.setItem('Authorization', info.token);
                localStorage.setItem('Username', info.username);
                localStorage.setItem('UserId', info.userId);
                navigate('/');
            }
        } catch (error) {
            message.error('用户名或密码错误');
        }
    };

    // 处理密码初始化表单提交
    const handlePasswordSubmit = async (event) => {
        event.preventDefault();
        const formData = new FormData(event.target);
        const password = formData.get('password');
        const confirmPassword = formData.get('confirm-password');

        if (password !== confirmPassword) {
            message.open({
                type: 'error',
                content: '两次输入的密码不一致',
            });
            return;
        }

        try {
            const params = {
                userid: 'admin',
                username: 'admin',
                email: 'admin@qq.com',
                phone: '18888888888',
                password: password,
                role: 'admin',
            };
            await registerUser(params);
            handleHideModal();
            window.location.reload();
        } catch (error) {
            console.error(error);
        }
    };

    const handleOidcLogin = async () => {
        try {
            const res = await getOidcInfo();
            if (res) {
                if (res.data.authType !== 2) {
                    message.error('OIDC 未启用，请联系管理员');
                    return;
                }

                const oidcConfig = {
                    authority: res.data.upperURI,
                    client_id: res.data.clientID,
                    redirect_uri: res.data.redirectURI,
                    response_type: 'code',
                    scope: 'openid profile email',
                };
                const userManager = new UserManager(oidcConfig);
                userManager.signinRedirect();
            }
        } catch (error) {
            console.error('获取 OIDC 信息失败:', error);
        }
    }

    // 显示/隐藏模态框
    const handleShowModal = () => setIsModalVisible(true);
    const handleHideModal = () => setIsModalVisible(false);


    return (
        <div className="min-h-screen flex bg-black text-white">
            {/* 左侧插画区 - 使用提供的背景图片 */}
            <div 
                className="hidden md:flex w-1/2 flex-col justify-center items-center p-12 relative overflow-hidden"
                style={{
                    background: 'linear-gradient(to bottom, #f8f9fa, #e9ecef)',
                }}
            >
                {/* 图片容器 - 居中显示，合适大小 */}
                <div className="relative flex flex-col items-center justify-center space-y-8">
                    {/* 标题和描述文字 */}
                    <motion.div
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ duration: 0.6 }}
                        className="text-center space-y-4"
                    >
                        <h2 className="text-4xl tracking-wide font-bold text-gray-800 mb-2">
                        WatchAlert 告警引擎
                        </h2>
                        <p className="text-gray-600 max-w-md text-lg leading-relaxed">
                            事件驱动运维,数据辅助决策。
                        </p>
                    </motion.div>
                    
                    {/* 图片显示区域 - 限制大小 */}
                    <motion.div
                        initial={{ opacity: 0, scale: 0.9 }}
                        animate={{ opacity: 1, scale: 1 }}
                        transition={{ duration: 0.6, delay: 0.2 }}
                        className="relative"
                        style={{
                            width: '100%',
                            maxWidth: '600px',
                            height: '400px',
                        }}
                    >
                        <img 
                            src={loginIllustration} 
                            alt="WatchAlert 告警引擎"
                            className="w-full h-full object-contain"
                            style={{
                                filter: 'drop-shadow(0 10px 30px rgba(0, 0, 0, 0.1))',
                            }}
                        />
                    </motion.div>
                </div>
            </div>

            {/* 右侧登录区域 */}
            <div className="w-full md:w-1/2 flex items-center justify-center px-6 py-12 bg-gradient-to-br from-slate-50 to-gray-100">
                <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.5 }}
                    className="bg-white text-black rounded-2xl shadow-2xl w-full max-w-md p-10 border border-gray-200"
                >
                    <h1 className="text-3xl font-bold mb-2 bg-gradient-to-r from-cyan-600 to-blue-600 bg-clip-text text-transparent">
                        欢迎回来
                    </h1>
                    <p className="text-gray-600 mb-8 text-base">请登录以继续使用 WatchAlert</p>
                    {!showOidcButtons ? (
                            <div>
                                <form onSubmit={onFinish} className="space-y-5">
                                    <div>
                                        <label className="block text-sm font-medium text-gray-700 mb-2">用户名</label>
                                        <input
                                            type="text"
                                            name="username"
                                            placeholder="请输入用户名"
                                            className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:outline-none focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500 focus:ring-opacity-20 hover:border-gray-400 transition-all"
                                            required
                                        />
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium text-gray-700 mb-2">密码</label>
                                        <input
                                            type="password"
                                            name="password"
                                            placeholder="请输入密码"
                                            className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:outline-none focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500 focus:ring-opacity-20 hover:border-gray-400 transition-all"
                                            required
                                        />
                                    </div>
                                    <div className="flex items-center justify-between pt-1">
                                        <label className="flex items-center space-x-2 cursor-pointer group">
                                            <input
                                                type="checkbox"
                                                className="form-checkbox h-4 w-4 text-black rounded border-gray-300"
                                            />
                                            <span className="text-sm text-gray-700 group-hover:text-black transition-colors">记住我</span>
                                        </label>
                                    </div>
                                    <button
                                        type="submit"
                                        className="w-full bg-gradient-to-r from-cyan-600 to-blue-600 text-white font-medium py-3.5 rounded-lg hover:from-cyan-700 hover:to-blue-700 active:scale-[0.98] transition-all shadow-lg hover:shadow-xl"
                                    >
                                        登录
                                    </button>
                                    {!passwordModal && (
                                        <button
                                            type="button"
                                            onClick={handleShowModal}
                                            className="text-sm text-gray-700 hover:text-black underline decoration-2 underline-offset-2 mt-3 transition-colors"
                                        >
                                            ➡️ 点击初始化 admin 密码
                                        </button>
                                    )}
                                </form>
                                <p className="text-gray-600 hover:text-cyan-600 text-center text-sm font-medium py-4 mt-4 cursor-pointer border-t border-gray-200 transition-colors" onClick={()=> setShowOidcButtons(true)}>使用 SSO 服务登录</p>
                            </div>
                        ):(
                            <div>
                                <button onClick={handleOidcLogin}
                                    className="w-full py-3.5 border-2 border-cyan-500 text-cyan-600 font-medium rounded-lg hover:bg-cyan-50 hover:border-cyan-600 active:scale-[0.98] transition-all text-center shadow-sm hover:shadow-md"
                                >
                                    使用 OIDC 登录
                                </button>
                                <p className="text-gray-600 hover:text-cyan-600 text-center text-sm font-medium py-4 mt-4 cursor-pointer border-t border-gray-200 transition-colors" onClick={()=> setShowOidcButtons(false)}>管理员登录</p>
                            </div>
                        )
                    }
                </motion.div>
            </div>

            {/* 密码初始化模态框 */}
            {isModalVisible && (
                <motion.div
                    initial={{ opacity: 0, scale: 0.9 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ opacity: 0, scale: 0.9 }}
                    transition={{ duration: 0.3 }}
                    className="fixed inset-0 bg-black bg-opacity-70 flex items-center justify-center z-50 backdrop-blur-sm"
                >
                    <div className="bg-white text-black p-10 rounded-2xl shadow-2xl max-w-md w-full mx-4">
                        <h2 className="text-2xl font-semibold mb-2">设置管理员密码</h2>
                        <p className="text-gray-700 mb-6 text-sm">首次使用需要为管理员账号设置密码</p>
                        <form onSubmit={handlePasswordSubmit} className="space-y-5">
                            <div>
                                <label htmlFor="init-password" className="block text-sm font-medium text-gray-800 mb-2">
                                    设置密码
                                </label>
                                <input
                                    type="password"
                                    id="init-password"
                                    name="password"
                                    className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:outline-none focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500 focus:ring-opacity-20 hover:border-gray-400 transition-all"
                                    required
                                />
                            </div>
                            <div>
                                <label htmlFor="confirm-password" className="block text-sm font-medium text-gray-800 mb-2">
                                    确认密码
                                </label>
                                <input
                                    type="password"
                                    id="confirm-password"
                                    name="confirm-password"
                                    className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:outline-none focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500 focus:ring-opacity-20 hover:border-gray-400 transition-all"
                                    required
                                />
                            </div>
                            <div className="flex justify-end space-x-3 pt-4">
                                <button
                                    type="button"
                                    onClick={handleHideModal}
                                    className="px-6 py-2.5 text-gray-700 hover:text-black font-medium transition-colors"
                                >
                                    取消
                                </button>
                                <button
                                    type="submit"
                                    className="px-6 py-2.5 bg-gradient-to-r from-cyan-600 to-blue-600 text-white font-medium rounded-lg hover:from-cyan-700 hover:to-blue-700 active:scale-[0.98] transition-all shadow-lg hover:shadow-xl"
                                >
                                    提交
                                </button>
                            </div>
                        </form>
                    </div>
                </motion.div>
            )}
        </div>
    );
};