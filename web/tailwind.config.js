/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    './utils/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  safelist: [
    // Skeleton loading colors used by BatchTestRunner
    'bg-pink-400',
    'bg-orange-400', 
    'bg-indigo-400',
    'bg-yellow-400',
    'bg-purple-400',
    'bg-teal-400',
    'bg-red-400',
    // Opacity variants
    'opacity-60',
    'opacity-50',
    'opacity-40', 
    'opacity-30',
    'opacity-100',
    // Animation classes
    'animate-pulse',
    'animate-spin',
    // Spinner border classes
    'border-2',
    'border-3', 
    'border-4',
    'border-current',
    'border-t-transparent',
    // Size classes
    'w-4',
    'h-4',
    'w-5',
    'h-5', 
    'w-6',
    'h-6',
    // Display and spacing
    'inline-block',
    'rounded-full',
    'mr-2',
    // Text and color classes
    'text-blue-600',
    'text-white',
    'sr-only',
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#eff6ff',
          100: '#dbeafe',
          200: '#bfdbfe',
          300: '#93c5fd',
          400: '#60a5fa',
          500: '#3b82f6',
          600: '#2563eb',
          700: '#1d4ed8',
          800: '#1e40af',
          900: '#1e3a8a',
        },
        // Custom test card colors
        'test-networking': '#b2f2bb',
        'test-l3': '#d0bfff',
        'test-l4': '#a5d8ff',
        'test-l7': '#eebefa',
        'test-dns': '#fcc2d7',
        'test-infrastructure': '#99e9f2',
        'test-results': '#ff8787',
      },
      fontFamily: {
        'poppins': ['Poppins', 'sans-serif'],
        'inter': ['Inter', 'sans-serif'],
        'comfortaa': ['Comfortaa', 'sans-serif'],
      },
      animation: {
        'fade-in': 'fadeIn 0.5s ease-in-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'bounce-subtle': 'bounceSubtle 2s infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideUp: {
          '0%': { transform: 'translateY(10px)', opacity: '0' },
          '100%': { transform: 'translateY(0)', opacity: '1' },
        },
        bounceSubtle: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-4px)' },
        },
      },
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
  ],
}
