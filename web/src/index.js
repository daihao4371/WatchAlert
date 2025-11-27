import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import App from './App.jsx';
import { ConfigProvider } from 'antd';
import zhCN from 'antd/es/locale/zh_CN.js';
import './pages/global.css';

ReactDOM.createRoot(document.getElementById('root')).render(
    <BrowserRouter
        future={{
            v7_startTransition: true,
            v7_relativeSplatPath: true,
        }}
    >
        <ConfigProvider componentSize='middle' locale={zhCN}>
            <App />
        </ConfigProvider>
    </BrowserRouter>
);
