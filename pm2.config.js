module.exports = {
  apps: [
    {
      name: 'myslotmate-backend',
      script: './myslotmate-backend',
      instances: 'max',
      exec_mode: 'cluster',
      env: {
        HTTP_PORT: 5000,
        NODE_ENV: 'production',
      },
      error_file: './logs/error.log',
      out_file: './logs/out.log',
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      merge_logs: true,
      max_memory_restart: '1G',
      autorestart: true,
      watch: false,
      ignore_watch: ['node_modules', 'logs', '.git'],
      max_restarts: 10,
      min_uptime: '10s',
      listen_timeout: 3000,
    },
  ],
};
