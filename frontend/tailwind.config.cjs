module.exports = {
  content: [
    './index.html',
    './src/**/*.{js,ts,svelte}',
    './node_modules/flowbite-svelte/**/*.{js,svelte,ts}'
  ],
  theme: {
    extend: {}
  },
  plugins: [require('flowbite/plugin')]
};
