/** @type {import('tailwindcss').Config} */
module.exports = {
  // Class-based, so the Alpine theme toggle in index.templ drives `dark:` variants.
  darkMode: "class",
  // Generated *_templ.go files and any Go code that builds class strings must be
  // scanned too, or those classes get purged from the bundle.
  content: [
    "./internal/views/**/*.templ",
    "./internal/**/*.go",
    "./utils/**/*.go",
  ],
  theme: {
    extend: {},
  },
  plugins: [],
};
