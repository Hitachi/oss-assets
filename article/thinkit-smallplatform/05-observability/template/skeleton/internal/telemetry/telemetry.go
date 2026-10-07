package telemetry

import (
	"context"
	"errors"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Setup はメトリクスとログの計装を初期化し、終了処理を返す。
// 送信先やサービス名は環境変数（OTEL_EXPORTER_OTLP_ENDPOINT /
// OTEL_SERVICE_NAME / OTEL_RESOURCE_ATTRIBUTES）から読み取られる。
func Setup(ctx context.Context) (func(context.Context) error, error) {
	// リソース属性を環境変数から構築する（service.name など）
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}

	// メトリクス：OTLP(gRPC)エクスポーターとMeterProviderをグローバルに設定
	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)
	otel.SetMeterProvider(meterProvider)

	// Goランタイムのメトリクス（メモリ・GCなど）を収集
	if err := runtime.Start(runtime.WithMeterProvider(meterProvider)); err != nil {
		return nil, err
	}

	// ログ：OTLP(gRPC)エクスポーターとLoggerProviderをグローバルに設定
	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	global.SetLoggerProvider(loggerProvider)

	// 標準ログ（log/slog）をOpenTelemetryへ橋渡しし、既定のロガーにする
	slog.SetDefault(otelslog.NewLogger("app"))

	// 両プロバイダーの終了処理をまとめて返す
	shutdown := func(ctx context.Context) error {
		return errors.Join(
			meterProvider.Shutdown(ctx),
			loggerProvider.Shutdown(ctx),
		)
	}
	return shutdown, nil
}
