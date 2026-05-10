/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  safelist: [
    "bg-blue-500",
    "bg-green-500",
    "bg-purple-500",
    "bg-red-500",
    "bg-orange-500",
    "bg-yellow-500",
    "text-blue-500",
    "text-green-500",
    "text-purple-500",
    "text-red-500",
    "text-orange-500",
    "text-yellow-500",
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: [
          "HarmonyOS Sans SC",
          "MiSans",
          "Alibaba PuHuiTi 3.0",
          "PingFang SC",
          "Microsoft YaHei",
          "sans-serif",
        ],
      },
    },
  },
  plugins: [],
};

