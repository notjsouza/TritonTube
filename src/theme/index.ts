import { createTheme } from '@mui/material/styles';

// UCSD Color Constants
export const ucsdColors = {
  navy: '#182B49',
  blue: '#00629B', 
  yellow: '#FFCD00',
  gold: '#C69214',
} as const;

export const theme = createTheme({
  palette: {
    primary: {
      main: ucsdColors.navy, // UC San Diego Navy
      light: ucsdColors.blue, // UC San Diego Blue
      dark: '#0F1A2E', // Darker version of Navy
    },
    secondary: {
      main: ucsdColors.yellow, // UC San Diego Yellow
      light: '#FFDD44', // Lighter yellow
      dark: ucsdColors.gold, // UC San Diego Gold
    },
    background: {
      default: '#f8f9fa',
      paper: '#ffffff',
    },
  },
  typography: {
    fontFamily: '"Roboto", "Helvetica", "Arial", sans-serif',
    h1: {
      fontSize: '2.5rem',
      fontWeight: 600,
    },
    h2: {
      fontSize: '2rem',
      fontWeight: 500,
    },
    h3: {
      fontSize: '1.5rem',
      fontWeight: 500,
    },
  },
  components: {
    MuiCard: {
      styleOverrides: {
        root: {
          borderRadius: 12,
          boxShadow: '0 2px 12px rgba(24, 43, 73, 0.08)',
          transition: 'transform 0.2s ease-in-out, box-shadow 0.2s ease-in-out',
          '&:hover': {
            transform: 'translateY(-2px)',
            boxShadow: '0 6px 20px rgba(24, 43, 73, 0.15)',
          },
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: {
          textTransform: 'none',
          borderRadius: 8,
          fontWeight: 500,
          padding: '8px 20px',
        },
        contained: {
          boxShadow: '0 2px 8px rgba(24, 43, 73, 0.2)',
          '&:hover': {
            boxShadow: '0 4px 12px rgba(24, 43, 73, 0.3)',
          },
        },
      },
    },
    MuiAppBar: {
      styleOverrides: {
        root: {
          backgroundColor: ucsdColors.navy,
          boxShadow: '0 2px 12px rgba(24, 43, 73, 0.15)',
        },
      },
    },
    MuiChip: {
      styleOverrides: {
        root: {
          borderRadius: 6,
        },
      },
    },
  },
});

export default theme;
