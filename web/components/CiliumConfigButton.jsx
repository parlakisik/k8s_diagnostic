export default function CiliumConfigButton({ 
  onConfigComplete, 
  disabled, 
  isRunning,
  onConfigClick 
}) {
  const handleConfigClick = () => {
    if (onConfigClick) {
      onConfigClick();
    }
  };

  return (
    <button
      onClick={handleConfigClick}
      disabled={disabled || isRunning}
      className={`rounded-xl font-comfortaa font-semibold transition-all ${
        disabled || isRunning
          ? 'opacity-50 cursor-not-allowed'
          : 'hover-lift card-shadow'
      }`}
      style={{ 
        padding: '15px', 
        borderRadius: '5px', 
        marginRight: '7px',
        background: 'linear-gradient(135deg, #fbbf24, #f59e0b)',
        color: 'white',
        border: 'none'
      }}
      title="Check Cilium configuration and get test recommendations"
    >
      {isRunning ? (
        <>
          <span className="animate-spin inline-block mr-2">🔍</span>
          Checking...
        </>
      ) : (
        <>🔧 Check Cilium Configs</>
      )}
    </button>
  );
}
