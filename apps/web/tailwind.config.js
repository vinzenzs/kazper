/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        // A dark, coach's-eye monitor palette.
        ink: {
          900: "#0b0f14",
          800: "#111722",
          700: "#1a2230",
          600: "#26303f",
        },
        accent: {
          DEFAULT: "#38bdf8",
          warn: "#fbbf24",
          danger: "#f87171",
          good: "#34d399",
        },
      },
      fontFamily: {
        mono: ["ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
    },
  },
  plugins: [],
};
