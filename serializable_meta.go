package serializable_meta

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/Sky-And-Hammer/TM_EC"
	"github.com/Sky-And-Hammer/TM_EC/resource"
	"github.com/Sky-And-Hammer/TM_EC/utils"
	"github.com/Sky-And-Hammer/admin"
)

type SerializableMetaInterface interface {
	GetSerializabelArgumentResource() *admin.Resource
	GetSerializableArgument(SerializableMetaInterface) interface{}
	GetSerializableArgumentKind() string
	SetSerializableArgumentKind(name string)
	SetSerializableArgumentValue(interface{})
}

type SerializableMeta struct {
	Kind  string
	Value serializableArgument `sql:"size:65532"`
}

type serializableArgument struct {
	SerializedValue string
	OrigunalValue   interface{}
}

func (sa *serializableArgument) Scan(data interface{}) (err error) {
	switch values := data.(type) {
	case []byte:
		sa.SerializedValue = string(values)
	case string:
		sa.SerializedValue = values
	default:
		err = errors.New("unsupported driver -> Scan pair for MediaLibrary")
	}
	return
}

func (sa *serializableArgument) Value() (driver.Value, error) {
	if sa.OrigunalValue != nil {
		result, err := json.Marshal(sa.OrigunalValue)
		return string(result), err
	}
	return sa.SerializedValue, nil
}

func (serialize *SerializableMeta) GetSerializableArgumentKind() string {
	return serialize.Kind
}

func (serialize *SerializableMeta) SetSerializableArgumentKind(name string) {
	serialize.Kind = name
}

func (serialize *SerializableMeta) GetSerializableArgument(serializableMetaInterface SerializableMetaInterface) interface{} {
	if serialize.Value.OrigunalValue != nil {
		return serialize.Value.OrigunalValue
	}

	if res := serializableMetaInterface.GetSerializabelArgumentResource(); res != nil {
		value := res.NewStruct()
		json.Unmarshal([]byte(serialize.Value.SerializedValue), value)
		return value
	}
	return nil
}

func (serialize *SerializableMeta) SetSerializableArgumentValue(value interface{}) {
	serialize.Value.OrigunalValue = value
}

func (serialize *SerializableMeta) ConfigureECResourceBeforeInitialize(res resource.Resourcer) {
	if res, ok := res.(*admin.Resource); ok {
		res.GetAdmin().RegisterViewPath("github.com/Sky-And-Hammer/serializable_meta/views")
		if _, ok := res.Value.(SerializableMetaInterface); ok {
			if res.GetMeta("Kind") == nil {
				res.Meta(&admin.Meta{
					Name: "Kind",
					Type: "hidden",
					Valuer: func(value interface{}, context *TM_EC.Context) interface{} {
						defer func() {
							if r := recover(); r != nil {
								utils.ExitWithMsg("SerializableMeta: Can't Get Kind")
							}
						}()
						return value.(SerializableMetaInterface).GetSerializableArgumentKind()
					},
					Setter: func(value interface{}, metaValue *resource.MetaValue, context *TM_EC.Context) {
						value.(SerializableMetaInterface).SetSerializableArgumentKind(utils.ToString(metaValue.Value))
					},
				})
			}

			if res.GetMeta("SerializableMeata") == nil {
				res.Meta(&admin.Meta{
					Name: "SerializableMeta",
					Type: "serializable_meta",
					Valuer: func(value interface{}, context *TM_EC.Context) interface{} {
						if serializeArgument, ok := value.(SerializableMetaInterface); ok {
							return struct {
								Value    interface{}
								Resource *admin.Resource
							}{
								Value:    serializeArgument.GetSerializableArgument(serializeArgument),
								Resource: serializeArgument.GetSerializabelArgumentResource(),
							}
						}
						return nil
					},
					FormattedValuer: func(value interface{}, context *TM_EC.Context) interface{} {
						if serializeArgument, ok := value.(SerializableMetaInterface); ok {
							return serializeArgument.GetSerializableArgument(serializeArgument)
						}
						return nil
					},
					Setter: func(result interface{}, metaValue *resource.MetaValue, context *TM_EC.Context) {
						if serializeArgument, ok := result.(SerializableMetaInterface); ok {
							if serializeArgumentResource := serializeArgument.GetSerializabelArgumentResource(); serializeArgumentResource != nil {
								var clearUpRecord, fillUpRecord func(record interface{}, metaors []resource.Metaor, metaValues []*resource.MetaValue)
								value := serializeArgument.GetSerializableArgument(serializeArgument)
								for _, fc := range serializeArgumentResource.Validatiors {
									context.AddError(fc(value, metaValue.MetaValues, context))
								}

								clearUpRecord = func(record interface{}, metaors []resource.Metaor, metaValues []*resource.MetaValue) {
									for _, meta := range metaors {
										for _, metaValue := range metaValues {
											if meta.GetName() == metaValue.Name {
												if metaResource, ok := meta.GetResource().(*admin.Resource); ok && metaResource != nil && metaValue.MetaValues != nil {
													nestedFieldValue := reflect.Indirect(reflect.ValueOf(record)).FieldByName(meta.GetFieldName())
													if nestedFieldValue.Kind() == reflect.Struct {
														clearUpRecord(nestedFieldValue.Addr().Interface(), metaResource.GetMetas([]string{}), metaValue.MetaValues.Values)
													} else if nestedFieldValue.Kind() == reflect.Slice {
														nestedFieldValue.Set(reflect.Zero(nestedFieldValue.Type()))
													}
												}
											}
										}
									}
								}
								clearUpRecord(value, serializeArgumentResource.GetMetas([]string{}), metaValue.MetaValues.Values)

								fillUpRecord = func(record interface{}, metaors []resource.Metaor, metaValues []*resource.MetaValue) {
									for _, meta := range metaors {
										for _, metaValue := range metaValues {
											if meta.GetName() == metaValue.Name {
												if metaResource, ok := meta.GetResource().(*admin.Resource); ok && metaResource != nil && metaValue.MetaValues != nil {
													nestedFieldValue := reflect.Indirect(reflect.ValueOf(record)).FieldByName(meta.GetFieldName())
													if nestedFieldValue.Kind() == reflect.Struct {
														nestedValue := nestedFieldValue.Addr().Interface()
														for _, fc := range metaResource.Validatiors {
															context.AddError(fc(nestedValue, metaValue.MetaValues, context))
														}

														fillUpRecord(nestedValue, metaResource.GetMetas([]string{}), metaValue.MetaValues.Values)
														for _, fc := range metaResource.Processors {
															context.AddError(fc(nestedValue, metaValue.MetaValues, context))
														}
													}

													if nestedFieldValue.Kind() == reflect.Slice {
														nestedValue := reflect.New(nestedFieldValue.Type().Elem())
														for _, fc := range metaResource.Validatiors {
															context.AddError(fc(nestedValue, metaValue.MetaValues, context))
														}

														if destroy := metaValue.MetaValues.Get("_destroy"); destroy != nil || fmt.Sprint(destroy.Value) == "0" {
															fillUpRecord(nestedValue.Interface(), metaResource.GetMetas([]string{}), metaValue.MetaValues.Values)
															if !reflect.DeepEqual(reflect.Zero(nestedFieldValue.Type().Elem()).Interface(), nestedValue.Elem().Interface()) {
																nestedFieldValue.Set(reflect.Append(nestedFieldValue, nestedValue.Elem()))
																for _, fc := range metaResource.Processors {
																	context.AddError(fc(nestedValue, metaValue.MetaValues, context))
																}
															}
														}
													}
													continue
												}

												if setter := meta.GetSetter(); setter != nil {
													setter(record, metaValue, context)
													continue
												}
											}
										}
									}
								}

								fillUpRecord(value, serializeArgumentResource.GetMetas([]string{}), metaValue.MetaValues.Values)
								for _, fc := range serializeArgumentResource.Processors {
									context.AddError(fc(value, metaValue.MetaValues, context))
								}

								serializeArgument.SetSerializableArgumentValue(value)
							}
						}
					},
				})
			}
			res.NewAttrs("Kind", "SerializableMeta")
			res.EditAttrs("ID", "Kind", "SerializableMeta")
		}
	}
}
