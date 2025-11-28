import React from 'react';
import { ConfigProvider, theme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { Helmet } from 'react-helmet';
import routes from './routes';
import { useRoutes } from 'react-router-dom';
import './index.css'
import { AppContextProvider } from './context/RuleContext';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';

// 全局配置 dayjs 使用中文 locale
dayjs.locale('zh-cn');

export default function App() {
    const element = useRoutes(routes);
    const title = "WatchAlert";

    return (
        <AppContextProvider>
            <ConfigProvider locale={zhCN} theme={{ algorithm: theme.defaultAlgorithm }}>
                <Helmet>
                    <title>{title}</title>
                </Helmet>
                {element}
            </ConfigProvider>
        </AppContextProvider>
    );
}