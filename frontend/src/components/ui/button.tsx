import * as React from "react";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "game" | "success" | "gold" | "outline" | "ghost" | "destructive";
  size?: "sm" | "md" | "lg";
}

const variantStyles: Record<string, string> = {
  game: "bg-accent text-accent-foreground shadow-3d hover:brightness-105 active:translate-y-1 active:shadow-none border-b-4 border-accent-shadow",
  success: "bg-success text-success-foreground shadow-3d-success hover:brightness-105 active:translate-y-1 active:shadow-none border-b-4 border-emerald-700",
  gold: "bg-gold text-slate-950 shadow-3d-gold hover:brightness-105 active:translate-y-1 active:shadow-none border-b-4 border-amber-600 font-extrabold",
  outline: "border border-border bg-card text-foreground hover:border-accent hover:text-accent shadow-sm",
  ghost: "text-muted-foreground hover:bg-card hover:text-foreground",
  destructive: "bg-danger text-danger-foreground border-b-4 border-rose-800 hover:brightness-105 active:translate-y-1 active:shadow-none",
};

const sizeStyles: Record<string, string> = {
  sm: "h-9 px-3 text-xs rounded-xl",
  md: "h-11 px-5 text-sm rounded-xl",
  lg: "h-13 px-7 text-base rounded-2xl",
};

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className = "", variant = "game", size = "md", children, disabled, ...props }, ref) => {
    return (
      <button
        ref={ref}
        disabled={disabled}
        className={`inline-flex items-center justify-center font-bold tracking-wide transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
        {...props}
      >
        {children}
      </button>
    );
  }
);
Button.displayName = "Button";
