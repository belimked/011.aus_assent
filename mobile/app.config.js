const config = require('./app.json').expo;
const push = process.env.AXLEO_IOS_PUSH === '1';
module.exports = {
  ...config,
  plugins: [
    ...config.plugins,
    ['expo-notifications', {enableRemoteNotifications: push, enableBackgroundRemoteNotifications: false, mode: 'production'}],
  ],
  extra: {...config.extra, iosPushEnabled: push},
};
